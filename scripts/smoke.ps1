param(
  [switch]$KeepWorkDir
)

$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.Net.Http
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location $Root

function Get-FreePort {
  $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Parse("127.0.0.1"), 0)
  $listener.Start()
  try {
    return $listener.LocalEndpoint.Port
  } finally {
    $listener.Stop()
  }
}

function New-MasterKey {
  $bytes = New-Object byte[] 32
  $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
  try {
    $rng.GetBytes($bytes)
  } finally {
    $rng.Dispose()
  }
  return [Convert]::ToBase64String($bytes)
}

function Wait-HttpOK($Uri) {
  for ($i = 0; $i -lt 80; $i++) {
    try {
      $resp = Invoke-WebRequest -UseBasicParsing -Uri $Uri -TimeoutSec 1
      if ($resp.StatusCode -eq 200) {
        return
      }
    } catch {
      Start-Sleep -Milliseconds 250
    }
  }
  throw "Timed out waiting for $Uri"
}

function Assert-AdminSecurityHeaders($Response) {
  if ($Response.Headers["X-Frame-Options"] -ne "DENY") {
    throw "Missing X-Frame-Options DENY on admin response"
  }
  if ($Response.Headers["X-Content-Type-Options"] -ne "nosniff") {
    throw "Missing X-Content-Type-Options nosniff on admin response"
  }
  if ($Response.Headers["Referrer-Policy"] -ne "no-referrer") {
    throw "Missing Referrer-Policy no-referrer on admin response"
  }
  $csp = [string]$Response.Headers["Content-Security-Policy"]
  if ($csp -notmatch "default-src 'self'" -or $csp -notmatch "frame-ancestors 'none'") {
    throw "Admin response has unexpected Content-Security-Policy: $csp"
  }
}

function Invoke-AdminProxyWriteSmoke {
  param(
    [int]$AdminPort,
    [string]$CookieHeader
  )

  $uri = "http://127.0.0.1:$AdminPort/admin/v1/pools/mock-chat/probe"
  $publicOrigin = "https://bridge.example.test"
  $publicHost = "bridge.example.test"

  $badResponse = Invoke-AdminProxyWriteRequest -Uri $uri -PublicOrigin $publicOrigin -PublicHost $publicHost -CookieHeader $CookieHeader
  if ($badResponse.StatusCode -ne 403) {
    throw "Expected proxied admin write without X-Forwarded-Proto to return 403, got $($badResponse.StatusCode): $($badResponse.Body)"
  }

  $goodResponse = Invoke-AdminProxyWriteRequest -Uri $uri -PublicOrigin $publicOrigin -PublicHost $publicHost -CookieHeader $CookieHeader -ForwardedProto "https"
  if ($goodResponse.StatusCode -ne 200) {
    throw "Expected proxied admin write with X-Forwarded-Proto to return 200, got $($goodResponse.StatusCode): $($goodResponse.Body)"
  }
  $probe = $goodResponse.Body | ConvertFrom-Json
  if (-not $probe.ok) {
    throw "Proxied admin write smoke did not return ok=true: $($probe | ConvertTo-Json -Compress)"
  }
  return $probe
}

function Invoke-AdminProxyWriteRequest {
  param(
    [string]$Uri,
    [string]$PublicOrigin,
    [string]$PublicHost,
    [string]$CookieHeader,
    [string]$ForwardedProto
  )

  $handler = [System.Net.Http.HttpClientHandler]::new()
  $handler.UseCookies = $false
  $client = [System.Net.Http.HttpClient]::new($handler)
  try {
    $request = [System.Net.Http.HttpRequestMessage]::new([System.Net.Http.HttpMethod]::Post, $Uri)
    $request.Headers.Host = $PublicHost
    $request.Headers.TryAddWithoutValidation("Origin", $PublicOrigin) | Out-Null
    $request.Headers.TryAddWithoutValidation("Cookie", $CookieHeader) | Out-Null
    if ($ForwardedProto) {
      $request.Headers.TryAddWithoutValidation("X-Forwarded-Proto", $ForwardedProto) | Out-Null
    }
    $request.Content = [System.Net.Http.StringContent]::new("{}", [System.Text.Encoding]::UTF8, "application/json")
    $response = $client.SendAsync($request).GetAwaiter().GetResult()
    return [pscustomobject]@{
      StatusCode = [int]$response.StatusCode
      Body = $response.Content.ReadAsStringAsync().GetAwaiter().GetResult()
    }
  } finally {
    $client.Dispose()
    $handler.Dispose()
  }
}

$work = Join-Path $env:TEMP ("sigilbridge-smoke-" + [guid]::NewGuid().ToString("n"))
New-Item -ItemType Directory -Force $work | Out-Null
$exe = Join-Path $work "sigilbridge.exe"
$apiPort = Get-FreePort
$adminPort = Get-FreePort
$adminToken = "admin_smoke"
$env:SIGILBRIDGE_MASTER_KEY = New-MasterKey

pnpm --dir ui install --frozen-lockfile
pnpm --dir ui run build
Remove-Item -Recurse -Force (Join-Path $Root "internal/admin/ui/dist") -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force (Join-Path $Root "internal/admin/ui/dist") | Out-Null
Copy-Item -Recurse -Force (Join-Path $Root "ui/dist/*") (Join-Path $Root "internal/admin/ui/dist/")
[System.IO.File]::WriteAllText((Join-Path $Root "internal/admin/ui/dist/.gitkeep"), "`n", [System.Text.UTF8Encoding]::new($false))
[System.IO.File]::WriteAllText((Join-Path $Root "internal/admin/ui/dist/assets/.gitkeep"), "`n", [System.Text.UTF8Encoding]::new($false))
go build -o $exe ./cmd/sigilbridge

$config = @"
server:
  bind: 127.0.0.1:$apiPort
admin:
  bind: 127.0.0.1:$adminPort
  tokens_file: admin_tokens.yaml
  ui_enabled: true
storage:
  path: data/sigilbridge.db
audit:
  enabled: true
  content_mode: hash
  retention_days: 30
  rotate_compress_after_days: 7
vault:
  master_key_env: SIGILBRIDGE_MASTER_KEY
logging:
  format: json
pools_file: pools.yaml
"@
[System.IO.File]::WriteAllText((Join-Path $work "config.yaml"), $config, [System.Text.UTF8Encoding]::new($false))

$adminTokens = @"
tokens:
  - name: smoke-admin
    token: $adminToken
"@
[System.IO.File]::WriteAllText((Join-Path $work "admin_tokens.yaml"), $adminTokens, [System.Text.UTF8Encoding]::new($false))

$pools = @"
pools:
  - name: mock-chat
    strategy: priority_first
    upstreams:
      - id: mock-1
        provider: mock
        priority: 1
        weight: 1
        config:
          input_tokens: 7
          output_tokens: 3
"@
[System.IO.File]::WriteAllText((Join-Path $work "pools.yaml"), $pools, [System.Text.UTF8Encoding]::new($false))

$proc = Start-Process -FilePath $exe -ArgumentList @("serve", "--config", (Join-Path $work "config.yaml")) -WorkingDirectory $work -PassThru -WindowStyle Hidden
try {
  Wait-HttpOK "http://127.0.0.1:$apiPort/healthz"

  $unauthStatus = 0
  try {
    Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$adminPort/admin/v1/pools" -TimeoutSec 2 | Out-Null
  } catch {
    $unauthStatus = [int]$_.Exception.Response.StatusCode
  }
  if ($unauthStatus -ne 401) {
    throw "Expected unauthenticated admin request to return 401, got $unauthStatus"
  }

  $session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
  $loginResponse = Invoke-WebRequest -UseBasicParsing -Method Post -ContentType "application/json" -WebSession $session -Uri "http://127.0.0.1:$adminPort/admin/v1/auth/login" -Body (@{ token = $adminToken } | ConvertTo-Json)
  $login = $loginResponse.Content | ConvertFrom-Json
  if (-not $login.ok) {
    throw "Admin login did not return ok=true"
  }
  $adminCookieHeader = @($loginResponse.Headers["Set-Cookie"]) | Where-Object { $_ -like "sigilbridge_admin=*" } | Select-Object -First 1
  if (-not $adminCookieHeader) {
    throw "Admin login response did not set sigilbridge_admin cookie"
  }
  $adminCookieHeader = ([string]$adminCookieHeader).Split(";")[0]
  $adminOriginHeaders = @{ Origin = "http://127.0.0.1:$adminPort" }

  $poolsResp = @(Invoke-RestMethod -WebSession $session -Uri "http://127.0.0.1:$adminPort/admin/v1/pools")
  if ($poolsResp.Count -ne 1 -or $poolsResp[0].id -ne "mock-chat") {
    throw "Unexpected pools response: $($poolsResp | ConvertTo-Json -Compress)"
  }
  $proxyWriteProbe = Invoke-AdminProxyWriteSmoke -AdminPort $adminPort -CookieHeader $adminCookieHeader

  $apiCredential = Invoke-RestMethod -WebSession $session -Method Post -ContentType "application/json" -Headers $adminOriginHeaders -Uri "http://127.0.0.1:$adminPort/admin/v1/credentials/api-key" -Body (@{
    provider = "openai_api"
    name = "smoke"
    api_key = "sk-smoke"
    model = "gpt-smoke"
    pool = "openai-smoke"
  } | ConvertTo-Json)
  if ($apiCredential.id -ne "vault://apikey/openai_api/smoke") {
    throw "API key credential import returned unexpected id: $($apiCredential | ConvertTo-Json -Compress)"
  }
  $apiCredentialList = Invoke-RestMethod -WebSession $session -Uri "http://127.0.0.1:$adminPort/admin/v1/credentials"
  if (@($apiCredentialList.api_keys).Count -lt 1) {
    throw "API key credential import did not appear in credentials list"
  }

  $sessionCredential = Invoke-RestMethod -WebSession $session -Method Post -ContentType "application/json" -Headers $adminOriginHeaders -Uri "http://127.0.0.1:$adminPort/admin/v1/credentials/session" -Body (@{
    provider = "claude_web"
    name = "smoke"
    user_agent = "SigilBridgeSmoke/1"
    organization_id = "org-smoke"
    cookies = @{ session = "s1" }
  } | ConvertTo-Json)
  if ($sessionCredential.id -ne "vault://claude_web/smoke") {
    throw "Session credential import returned unexpected id: $($sessionCredential | ConvertTo-Json -Compress)"
  }
  $credentialsResp = Invoke-RestMethod -WebSession $session -Uri "http://127.0.0.1:$adminPort/admin/v1/credentials"
  if (@($credentialsResp.sessions).Count -lt 1) {
    throw "Session credential import did not appear in credentials list"
  }
  $sessionDeleted = Invoke-RestMethod -WebSession $session -Method Delete -Headers $adminOriginHeaders -Uri "http://127.0.0.1:$adminPort/admin/v1/credentials?id=$([uri]::EscapeDataString($sessionCredential.id))"
  if (-not $sessionDeleted.ok) {
    throw "Session credential delete did not return ok=true"
  }

  $keyCreateBody = @{
    prefix = "test"
    metadata = @{ name = "smoke" }
    scopes = @{
      allowed_pools = @("mock-chat")
      allowed_models = @("mock-chat")
      ip_allowlist = @("127.0.0.1/32")
    }
    budgets = @{
      daily_cents = 50
      monthly_cents = 500
      hard_cap = $true
    }
    rate_limits = @{
      rpm = 60
      tpm = 12000
    }
  } | ConvertTo-Json -Depth 6
  $created = Invoke-RestMethod -WebSession $session -Method Post -ContentType "application/json" -Headers $adminOriginHeaders -Uri "http://127.0.0.1:$adminPort/admin/v1/keys" -Body $keyCreateBody
  if (-not $created.id -or -not $created.plaintext.StartsWith("sb_test_")) {
    throw "Key create response missing id/plaintext"
  }
  if ($created.scopes.allowed_pools[0] -ne "mock-chat" -or $created.budgets.daily_cents -ne 50 -or $created.rate_limits.rpm -ne 60) {
    throw "Scoped key create did not persist scopes/budgets/rate limits: $($created | ConvertTo-Json -Compress -Depth 6)"
  }

  $chatHeaders = @{ Authorization = "Bearer $($created.plaintext)" }
  $chatBody = '{"model":"mock-chat","messages":[{"role":"user","content":"hello"}]}'
  $chat = Invoke-RestMethod -Method Post -ContentType "application/json" -Headers $chatHeaders -Uri "http://127.0.0.1:$apiPort/v1/chat/completions" -Body $chatBody
  if (-not $chat.id -or -not $chat.choices) {
    throw "OpenAI-compatible mock dispatch failed"
  }

  $revoked = Invoke-RestMethod -WebSession $session -Method Patch -ContentType "application/json" -Headers $adminOriginHeaders -Uri "http://127.0.0.1:$adminPort/admin/v1/keys/$($created.id)" -Body '{"revoked":true}'
  if (-not $revoked.revoked_at) {
    throw "Key revoke did not set revoked_at"
  }

  $deleted = Invoke-RestMethod -WebSession $session -Method Delete -Headers $adminOriginHeaders -Uri "http://127.0.0.1:$adminPort/admin/v1/keys/$($created.id)"
  if (-not $deleted.ok) {
    throw "Key delete did not return ok=true"
  }

  $ui = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$adminPort/admin/ui/"
  if ($ui.StatusCode -ne 200 -or $ui.Content -notmatch "SigilBridge") {
    throw "Embedded admin UI did not serve expected HTML"
  }
  Assert-AdminSecurityHeaders $ui
  $assetPath = [regex]::Match($ui.Content, '/admin/ui/assets/[^"]+\.js').Value
  if (-not $assetPath) {
    throw "Embedded admin UI HTML did not reference a hashed JS asset"
  }
  $asset = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$adminPort$assetPath"
  $assetCache = [string]$asset.Headers["Cache-Control"]
  if ($assetCache -notmatch "immutable") {
    throw "Embedded admin UI asset missing immutable cache header: $assetCache"
  }

  $auditFile = Join-Path $work ("audit\" + (Get-Date).ToUniversalTime().ToString("yyyy-MM-dd") + ".jsonl")
  if (-not (Test-Path $auditFile)) {
    throw "Expected audit file $auditFile"
  }

  [pscustomobject]@{
    Api = "http://127.0.0.1:$apiPort"
    Admin = "http://127.0.0.1:$adminPort"
    UnauthStatus = $unauthStatus
    Pools = $poolsResp.Count
    ProxiedWrite = [bool]$proxyWriteProbe.ok
    SessionCredential = $sessionCredential.id
    CreatedKey = $created.id
    ChatID = $chat.id
    Revoked = [bool]$revoked.revoked_at
    Deleted = [bool]$deleted.ok
    UiStatus = $ui.StatusCode
    AuditFile = $auditFile
  } | Format-List
} finally {
  if ($proc -and -not $proc.HasExited) {
    Stop-Process -Id $proc.Id -Force
  }
  if (-not $KeepWorkDir) {
    Remove-Item -Recurse -Force $work -ErrorAction SilentlyContinue
  } else {
    Write-Host "Smoke work dir kept at $work"
  }
}
