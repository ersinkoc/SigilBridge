param(
  [switch]$Race,
  [switch]$Coverage,
  [switch]$SkipUI,
  [switch]$Smoke,
  [switch]$DockerSmoke
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $Root

if ($Race) {
  $CanRunNativeRace = $true
  if ($IsWindows -or $env:OS -eq "Windows_NT") {
    $CanRunNativeRace = [bool](Get-Command gcc -ErrorAction SilentlyContinue)
  }
  if ($CanRunNativeRace) {
    $env:CGO_ENABLED = "1"
    go test -race ./...
  } else {
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
      throw "go test -race requires gcc on Windows. Install gcc or Docker, then rerun .\test.ps1 -Race."
    }
    docker run --rm `
      -v "${Root}:/src" `
      -w /src `
      -e GOCACHE=/tmp/gocache `
      -e GOMODCACHE=/tmp/gomodcache `
      golang:1.26.3 `
      bash -lc "/usr/local/go/bin/go test -race ./..."
  }
} else {
  go test ./...
}

go build ./...
go vet ./...

if ($Coverage) {
  go test "-coverprofile=coverage.out" ./internal/router/... ./internal/ir/... ./internal/budget/... ./internal/oauth/... ./internal/cliacp/...
  go tool cover "-func=coverage.out"
}

if (-not $SkipUI) {
  pnpm --dir ui install --frozen-lockfile
  pnpm --dir ui run lint
  pnpm --dir ui run test
  pnpm --dir ui run build
}

if ($Smoke) {
  & (Join-Path $Root "scripts/smoke.ps1")
}

if ($DockerSmoke) {
  & (Join-Path $Root "scripts/docker-smoke.ps1")
}
