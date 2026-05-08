param(
  [switch]$RuntimeState
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $Root

function Backup-LocalState {
  $statePaths = @(
    "config.yaml",
    "pools.yaml",
    "oauth_providers.yaml",
    "admin_tokens.yaml",
    ".sigilbridge",
    "data",
    "backup",
    "audit",
    "examples/data",
    "examples/backup",
    "examples/audit"
  ) | Where-Object { Test-Path (Join-Path $Root $_) }

  if ($statePaths.Count -eq 0) {
    return $null
  }

  $backupDir = Join-Path $Root "artifacts/local-state-backups"
  New-Item -ItemType Directory -Force $backupDir | Out-Null
  $stamp = (Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmssZ")
  $backupPath = Join-Path $backupDir "sigilbridge-local-state-$stamp.zip"
  $literalPaths = $statePaths | ForEach-Object { Join-Path $Root $_ }
  Compress-Archive -LiteralPath $literalPaths -DestinationPath $backupPath -Force
  return $backupPath
}

Remove-Item -Recurse -Force dist, ui/dist, ui/.lighthouseci, ui/test-results, .lighthouseci -ErrorAction SilentlyContinue
Remove-Item -Force coverage.out -ErrorAction SilentlyContinue
Remove-Item -Force sigilbridge, sigilbridge.exe -ErrorAction SilentlyContinue

if ($RuntimeState) {
  $backupPath = Backup-LocalState
  if ($backupPath) {
    Write-Host "Backed up local runtime state to $backupPath"
  }
  Remove-Item -Recurse -Force data, backup, audit, examples/data, examples/backup, examples/audit -ErrorAction SilentlyContinue
}

Remove-Item -Recurse -Force internal/admin/ui/dist -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force internal/admin/ui/dist/assets | Out-Null
[System.IO.File]::WriteAllText(
  (Join-Path $Root "internal/admin/ui/dist/.gitkeep"),
  "`n",
  [System.Text.UTF8Encoding]::new($false)
)
[System.IO.File]::WriteAllText(
  (Join-Path $Root "internal/admin/ui/dist/assets/.gitkeep"),
  "`n",
  [System.Text.UTF8Encoding]::new($false)
)

if ($RuntimeState) {
  Write-Host "Cleaned generated artifacts and local runtime state."
} else {
  Write-Host "Cleaned generated artifacts. Local runtime state was preserved; pass -RuntimeState to remove data, backup, and audit directories."
}
