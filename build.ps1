param(
  [string]$Version = "dev",
  [switch]$SkipUI,
  [switch]$SkipTests
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $Root

function Require-Command($Name) {
  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    throw "$Name is required but was not found in PATH"
  }
}

Require-Command go
if (-not $SkipUI) {
  Require-Command pnpm
}

$Commit = "unknown"
try {
  $Commit = (git rev-parse --short=12 HEAD 2>$null)
  if (-not $Commit) { $Commit = "unknown" }
} catch {
  $Commit = "unknown"
}
$BuildDate = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")

if (-not $SkipTests) {
  go test ./...
  go vet ./...
  if (-not $SkipUI) {
    pnpm --dir ui install --frozen-lockfile
    pnpm --dir ui run lint
    pnpm --dir ui run test
  }
}

if (-not $SkipUI) {
  pnpm --dir ui install --frozen-lockfile
  pnpm --dir ui run build
  Remove-Item -Recurse -Force internal/admin/ui/dist -ErrorAction SilentlyContinue
  New-Item -ItemType Directory -Force internal/admin/ui/dist | Out-Null
  Copy-Item -Recurse -Force ui/dist/* internal/admin/ui/dist/
}

New-Item -ItemType Directory -Force dist | Out-Null
$Out = "dist/sigilbridge"
if ($IsWindows -or $env:OS -eq "Windows_NT") {
  $Out = "dist/sigilbridge.exe"
}
$Ldflags = "-s -w -X main.version=$Version -X main.commit=$Commit -X main.date=$BuildDate"
go build -trimpath -ldflags $Ldflags -o $Out ./cmd/sigilbridge

Write-Host "Built $Out"
