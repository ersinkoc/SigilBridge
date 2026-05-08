param(
  [string]$Config = ".sigilbridge/config.yaml",
  [int]$UiPort = 8189,
  [switch]$NoUI,
  [switch]$CreateKey
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $Root

function Require-Command($Name) {
  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    throw "$Name is required but was not found in PATH"
  }
}

function Get-NativeCommand($Name) {
  foreach ($candidate in @("$Name.cmd", "$Name.exe", $Name)) {
    $cmd = Get-Command $candidate -ErrorAction SilentlyContinue
    if ($cmd -and $cmd.Source) {
      return $cmd.Source
    }
  }
  throw "$Name is required but was not found in PATH"
}

function Get-ConfigBind($Path, $Section, $Fallback) {
  if (-not (Test-Path -LiteralPath $Path)) {
    return $Fallback
  }
  $current = ""
  foreach ($line in Get-Content -LiteralPath $Path) {
    if ($line -match '^([A-Za-z_]+):\s*$') {
      $current = $matches[1]
      continue
    }
    if ($current -eq $Section -and $line -match '^\s+bind:\s*(.+?)\s*$') {
      return ($matches[1].Trim("`"", "'"))
    }
  }
  return $Fallback
}

function Set-ConfigBind($Path, $Section, $Bind) {
  $lines = [System.Collections.Generic.List[string]]::new()
  $lines.AddRange([string[]](Get-Content -LiteralPath $Path))
  $current = ""
  for ($i = 0; $i -lt $lines.Count; $i++) {
    $line = $lines[$i]
    if ($line -match '^([A-Za-z_]+):\s*$') {
      $current = $matches[1]
      continue
    }
    if ($current -eq $Section -and $line -match '^(\s+)bind:\s*.+?\s*$') {
      $lines[$i] = "$($matches[1])bind: $Bind"
      [System.IO.File]::WriteAllLines($Path, $lines, [System.Text.UTF8Encoding]::new($false))
      return
    }
  }
  throw "Could not find $Section.bind in $Path"
}

function Convert-BindToHttpBase($Bind) {
  $hostPart = "127.0.0.1"
  $portPart = "8187"
  if ($Bind -match '^\[([^\]]+)\]:(\d+)$') {
    $hostPart = $matches[1]
    $portPart = $matches[2]
  } elseif ($Bind -match '^([^:]*):(\d+)$') {
    $hostPart = $matches[1]
    $portPart = $matches[2]
  }
  if ($hostPart -eq "" -or $hostPart -eq "0.0.0.0" -or $hostPart -eq "::") {
    $hostPart = "127.0.0.1"
  }
  if ($hostPart.Contains(":") -and -not $hostPart.StartsWith("[")) {
    $hostPart = "[$hostPart]"
  }
  return "http://${hostPart}:$portPart"
}

Require-Command go
if (-not $NoUI) {
  Require-Command pnpm
}

if ([System.IO.Path]::IsPathRooted($Config)) {
  $ConfigPath = [System.IO.Path]::GetFullPath($Config)
} else {
  $ConfigPath = [System.IO.Path]::GetFullPath((Join-Path $Root $Config))
}
if (-not (Test-Path -LiteralPath $ConfigPath)) {
  $configName = [System.IO.Path]::GetFileName($ConfigPath)
  if ($configName -ne "config.yaml") {
    throw "Config '$Config' does not exist. Use a path ending in config.yaml so dev.ps1 can initialize its directory, or run sigilbridge init yourself."
  }
  $ConfigDir = Split-Path -Parent $ConfigPath
  Write-Host "Config not found. Initializing local dev config in $ConfigDir"
  go run ./cmd/sigilbridge init --dir $ConfigDir | Write-Host
}

$ConfigDir = Split-Path -Parent $ConfigPath
$DevMasterKeyPath = Join-Path $ConfigDir "master.key"
if (-not $env:SIGILBRIDGE_MASTER_KEY) {
  if (Test-Path -LiteralPath $DevMasterKeyPath) {
    $env:SIGILBRIDGE_MASTER_KEY = (Get-Content -LiteralPath $DevMasterKeyPath -Raw).Trim()
    Write-Host "Loaded SIGILBRIDGE_MASTER_KEY from $DevMasterKeyPath."
  } else {
    $bytes = New-Object byte[] 32
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    try {
      $rng.GetBytes($bytes)
    } finally {
      $rng.Dispose()
    }
    $env:SIGILBRIDGE_MASTER_KEY = [Convert]::ToBase64String($bytes)
    New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null
    [System.IO.File]::WriteAllText($DevMasterKeyPath, $env:SIGILBRIDGE_MASTER_KEY, [System.Text.UTF8Encoding]::new($false))
    Write-Host "Generated persistent dev SIGILBRIDGE_MASTER_KEY at $DevMasterKeyPath."
  }
}

$serverBind = Get-ConfigBind $ConfigPath "server" "127.0.0.1:8187"
$adminBind = Get-ConfigBind $ConfigPath "admin" "127.0.0.1:8188"
if ($serverBind -eq "127.0.0.1:8787") {
  $serverBind = "127.0.0.1:8187"
  Set-ConfigBind $ConfigPath "server" $serverBind
}
if ($adminBind -eq "127.0.0.1:8788") {
  $adminBind = "127.0.0.1:8188"
  Set-ConfigBind $ConfigPath "admin" $adminBind
}

$BackendBase = Convert-BindToHttpBase $serverBind
$AdminBase = Convert-BindToHttpBase $adminBind

if ($CreateKey) {
  $keyOut = go run ./cmd/sigilbridge keys create test --config $ConfigPath --name dev
  $key = ($keyOut -split "`r?`n")[0]
  Write-Host "Dev bridge key: $key"
}

$backendArgs = @("run", "./cmd/sigilbridge", "serve", "--config", $ConfigPath)
$backend = Start-Process -FilePath "go" -ArgumentList $backendArgs -PassThru -NoNewWindow

$ui = $null
try {
  if (-not $NoUI) {
    pnpm --dir ui install --frozen-lockfile
    $env:VITE_SIGILBRIDGE_TARGET = $BackendBase
    $env:VITE_SIGILBRIDGE_ADMIN_TARGET = $AdminBase
    $pnpm = Get-NativeCommand "pnpm"
    $uiArgs = @("--dir", "ui", "run", "dev", "--", "--host", "127.0.0.1", "--port", "$UiPort", "--strictPort")
    $ui = Start-Process -FilePath $pnpm -ArgumentList $uiArgs -PassThru -NoNewWindow
    Write-Host "Dev UI:          http://127.0.0.1:$UiPort"
  }
  Write-Host "Backend API:     $BackendBase/v1"
  Write-Host "Embedded Admin:  $AdminBase/admin/ui/"
  Write-Host "Press Ctrl+C to stop."
  while (-not $backend.HasExited -and ($NoUI -or -not $ui.HasExited)) {
    Start-Sleep -Seconds 1
  }
} finally {
  if ($ui -and -not $ui.HasExited) {
    Stop-Process -Id $ui.Id -Force
  }
  if ($backend -and -not $backend.HasExited) {
    Stop-Process -Id $backend.Id -Force
  }
}
