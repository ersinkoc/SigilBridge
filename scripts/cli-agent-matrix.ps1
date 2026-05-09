param(
  [string]$AdminUrl = "http://127.0.0.1:8788",
  [string]$AdminToken = $env:SIGILBRIDGE_ADMIN_TOKEN,
  [string[]]$Providers = @(),
  [switch]$Enable,
  [switch]$Probe,
  [switch]$IncludeMissing,
  [switch]$AllowThirdPartyNpx,
  [string]$OutputPath
)

$ErrorActionPreference = "Stop"

function Join-AdminUrl([string]$Base, [string]$Path) {
  return $Base.TrimEnd("/") + "/" + $Path.TrimStart("/")
}

function Invoke-AdminJson {
  param(
    [Parameter(Mandatory=$true)][string]$Path,
    [string]$Method = "GET",
    [object]$Body = $null,
    [Microsoft.PowerShell.Commands.WebRequestSession]$Session
  )
  $headers = @{}
  if ($Method -ne "GET") {
    $headers["Origin"] = $AdminUrl.TrimEnd("/")
  }
  $params = @{
    Method = $Method
    Uri = (Join-AdminUrl $AdminUrl $Path)
    Headers = $headers
  }
  if ($Session) {
    $params.WebSession = $Session
  }
  if ($null -ne $Body) {
    $params.ContentType = "application/json"
    $params.Body = ($Body | ConvertTo-Json -Depth 8)
  }
  return Invoke-RestMethod @params
}

function New-ResultRow($Agent) {
  return [ordered]@{
    provider = [string]$Agent.provider
    name = [string]$Agent.name
    source = [string]$Agent.source
    version = [string]$Agent.version
    command = [string]$Agent.command
    args = @($Agent.args)
    protocol = [string]$Agent.protocol
    framing = [string]$Agent.framing
    available = [bool]$Agent.available
    configured = [bool]$Agent.configured
    path = [string]$Agent.path
    pool = $null
    upstream = $null
    status = "detected"
    enable_ok = $null
    probe_ok = $null
    checked = $null
    passed = $null
    error = [string]$Agent.error
  }
}

if ([string]::IsNullOrWhiteSpace($AdminToken)) {
  throw "Admin token is required. Pass -AdminToken or set SIGILBRIDGE_ADMIN_TOKEN."
}

$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
Invoke-AdminJson -Session $session -Method "POST" -Path "/admin/v1/auth/login" -Body @{ token = $AdminToken } | Out-Null

$status = Invoke-AdminJson -Session $session -Path "/admin/v1/credentials/cli"
$detect = Invoke-AdminJson -Session $session -Path "/admin/v1/credentials/cli/detect"

$configuredByProvider = @{}
foreach ($agent in @($status.agents)) {
  if ($agent.provider) {
    $configuredByProvider[[string]$agent.provider] = $agent
  }
}

$providerSet = @{}
foreach ($provider in $Providers) {
  if (-not [string]::IsNullOrWhiteSpace($provider)) {
    $providerSet[$provider] = $true
  }
}

$rows = @()
foreach ($agent in @($detect.agents)) {
  $provider = [string]$agent.provider
  if ($providerSet.Count -gt 0 -and -not $providerSet.ContainsKey($provider)) {
    continue
  }
  if (-not $IncludeMissing -and -not [bool]$agent.available) {
    continue
  }

  $row = New-ResultRow $agent
  if ($configuredByProvider.ContainsKey($provider)) {
    $configured = $configuredByProvider[$provider]
    $row.configured = $true
    $row.pool = [string]$configured.pool
    $row.upstream = [string]$configured.upstream
  }

  $isThirdPartyNpx = ([string]$agent.source -eq "ACP registry" -and [string]$agent.command -eq "npx")
  if (($Enable -or $Probe) -and $isThirdPartyNpx -and -not $AllowThirdPartyNpx) {
    $row.status = "skipped_npx"
    $row.error = "Pass -AllowThirdPartyNpx to enable/probe registry agents that run through npx."
    $rows += [pscustomobject]$row
    continue
  }

  if ($Enable -and [bool]$agent.available) {
    try {
      $enableBody = @{
        provider = $provider
        command = [string]$agent.command
        protocol = [string]$agent.protocol
        framing = [string]$agent.framing
        args = @($agent.args)
      }
      $enabled = Invoke-AdminJson -Session $session -Method "POST" -Path "/admin/v1/credentials/cli/enable" -Body $enableBody
      $row.enable_ok = [bool]$enabled.ok
      $row.pool = [string]$enabled.pool
      $row.upstream = [string]$enabled.upstream
      $row.configured = [bool]$enabled.ok
      $row.status = if ([bool]$enabled.ok) { "enabled" } else { "enable_failed" }
    } catch {
      $row.enable_ok = $false
      $row.status = "enable_failed"
      $row.error = $_.Exception.Message
    }
  }

  if ($Probe -and -not [string]::IsNullOrWhiteSpace([string]$row.pool)) {
    try {
      $probeResult = Invoke-AdminJson -Session $session -Method "POST" -Path "/admin/v1/pools/$([uri]::EscapeDataString([string]$row.pool))/probe" -Body @{}
      $row.probe_ok = [bool]$probeResult.ok
      $row.checked = [int]$probeResult.checked
      $row.passed = [int]$probeResult.passed
      $row.status = if ([bool]$probeResult.ok) { "passed" } else { "probe_failed" }
      if (-not [bool]$probeResult.ok) {
        $firstError = @($probeResult.results | Where-Object { -not $_.ok } | Select-Object -First 1)
        if ($firstError.Count -gt 0) {
          $row.error = [string]$firstError[0].error
        }
      }
    } catch {
      $row.probe_ok = $false
      $row.status = "probe_failed"
      $row.error = $_.Exception.Message
    }
  }

  $rows += [pscustomobject]$row
}

$summary = [ordered]@{
  generated_at = (Get-Date).ToUniversalTime().ToString("o")
  admin_url = $AdminUrl.TrimEnd("/")
  cli_subsystem_enabled = [bool]$status.enabled
  scanned = @($rows).Count
  available = @($rows | Where-Object { $_.available }).Count
  configured = @($rows | Where-Object { $_.configured }).Count
  passed = @($rows | Where-Object { $_.probe_ok -eq $true }).Count
  failed = @($rows | Where-Object { $_.status -match "failed" }).Count
  skipped = @($rows | Where-Object { $_.status -like "skipped*" }).Count
  results = $rows
}

$json = $summary | ConvertTo-Json -Depth 12
if (-not [string]::IsNullOrWhiteSpace($OutputPath)) {
  $parent = Split-Path -Parent $OutputPath
  if (-not [string]::IsNullOrWhiteSpace($parent)) {
    New-Item -ItemType Directory -Force $parent | Out-Null
  }
  [System.IO.File]::WriteAllText($OutputPath, $json + [Environment]::NewLine, [System.Text.UTF8Encoding]::new($false))
}

$summary.results | Format-Table provider, source, available, configured, status, pool, error -AutoSize
Write-Host ""
Write-Host "Scanned=$($summary.scanned) Available=$($summary.available) Configured=$($summary.configured) Passed=$($summary.passed) Failed=$($summary.failed) Skipped=$($summary.skipped)"
if ($OutputPath) {
  Write-Host "Report=$OutputPath"
}
