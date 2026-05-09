param(
  [Parameter(Mandatory = $true)]
  [string]$AdminUrl,
  [string]$AdminToken = $env:SIGILBRIDGE_ADMIN_TOKEN,
  [switch]$AllowHttp,
  [switch]$Quiet
)

$ErrorActionPreference = "Stop"

function Get-BaseOrigin {
  param([string]$Value)

  $uri = [uri]$Value
  if (-not $uri.IsAbsoluteUri) {
    throw "AdminUrl must be an absolute URL, for example https://bridge.example.com"
  }
  if ($uri.Scheme -ne "https" -and -not $AllowHttp) {
    throw "AdminUrl must use https. Pass -AllowHttp only for private/local preflight checks."
  }
  if ($uri.Scheme -ne "https" -and $uri.Scheme -ne "http") {
    throw "AdminUrl must use http or https"
  }
  return $uri.GetLeftPart([System.UriPartial]::Authority)
}

function Join-AdminUrl {
  param(
    [string]$Origin,
    [string]$Path
  )
  return "$($Origin.TrimEnd('/'))/$($Path.TrimStart('/'))"
}

function Get-ErrorResponseBody {
  param($Response)

  if (-not $Response) {
    return ""
  }
  try {
    $stream = $Response.GetResponseStream()
    if (-not $stream) {
      return ""
    }
    $reader = [System.IO.StreamReader]::new($stream)
    try {
      return $reader.ReadToEnd()
    } finally {
      $reader.Dispose()
    }
  } catch {
    return ""
  }
}

if ([string]::IsNullOrWhiteSpace($AdminToken)) {
  throw "AdminToken is required. Pass -AdminToken or set SIGILBRIDGE_ADMIN_TOKEN."
}

$origin = Get-BaseOrigin $AdminUrl
$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession

$login = Invoke-WebRequest -UseBasicParsing -Method Post -ContentType "application/json" -WebSession $session -Uri (Join-AdminUrl $origin "/admin/v1/auth/login") -Body (@{ token = $AdminToken } | ConvertTo-Json)
$loginBody = $login.Content | ConvertFrom-Json
if (-not $loginBody.ok) {
  throw "Admin login did not return ok=true"
}

$health = Invoke-WebRequest -UseBasicParsing -WebSession $session -Uri (Join-AdminUrl $origin "/admin/v1/health")
if ($health.StatusCode -ne 200) {
  throw "Admin health returned unexpected status $($health.StatusCode)"
}

$reloadStatus = 0
$reloadBody = ""
try {
  $reload = Invoke-WebRequest -UseBasicParsing -WebSession $session -Method Post -ContentType "application/json" -Headers @{ Origin = $origin } -Uri (Join-AdminUrl $origin "/admin/v1/reload") -Body "{}"
  $reloadStatus = [int]$reload.StatusCode
  $reloadBody = [string]$reload.Content
} catch {
  if ($_.Exception.Response) {
    $reloadStatus = [int]$_.Exception.Response.StatusCode
    $reloadBody = Get-ErrorResponseBody $_.Exception.Response
  } else {
    throw
  }
}

if ($reloadStatus -eq 403) {
  throw "Cookie-authenticated admin write returned 403. Check that the reverse proxy preserves Host and sets X-Forwarded-Proto to the public scheme."
}
if ($reloadStatus -ne 200 -and $reloadStatus -ne 409) {
  throw "Admin reload preflight returned unexpected status $reloadStatus. Body: $reloadBody"
}

$reloadResult = $null
if (-not [string]::IsNullOrWhiteSpace($reloadBody)) {
  try {
    $reloadResult = $reloadBody | ConvertFrom-Json
  } catch {
    $reloadResult = $null
  }
}

$result = [pscustomobject]@{
  Admin = $origin
  LoginOK = [bool]$loginBody.ok
  HealthStatus = [int]$health.StatusCode
  SameOriginWrite = $true
  ReloadStatus = $reloadStatus
  RestartRequiredFields = if ($reloadResult -and $reloadResult.restart_required_fields) { @($reloadResult.restart_required_fields) } else { @() }
}

if (-not $Quiet) {
  $result | Format-List
}
