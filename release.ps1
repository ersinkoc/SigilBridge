param(
  [string]$Version = "dev",
  [switch]$SkipUIBuild,
  [switch]$PublishDocker
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $Root

function Get-NativeCommand([string[]]$Names) {
  foreach ($name in $Names) {
    $cmd = Get-Command $name -ErrorAction SilentlyContinue
    if ($cmd) {
      return $cmd.Source
    }
  }
  return $null
}

$go = Get-NativeCommand @("go.exe", "go")
if (-not $go) {
  throw "go is required to build release artifacts."
}
$tar = Get-NativeCommand @("tar.exe", "tar")
if (-not $tar) {
  throw "tar is required to create release archives."
}

if (-not $SkipUIBuild) {
  $pnpm = Get-NativeCommand @("pnpm.cmd", "pnpm.exe", "pnpm")
  if (-not $pnpm) {
    throw "pnpm is required to build the embedded UI."
  }
  & $pnpm --dir ui install --frozen-lockfile
  if ($LASTEXITCODE -ne 0) { throw "pnpm install failed with exit code $LASTEXITCODE" }
  & $pnpm --dir ui run build
  if ($LASTEXITCODE -ne 0) { throw "pnpm build failed with exit code $LASTEXITCODE" }

  $embedDist = Join-Path $Root "internal/admin/ui/dist"
  if (Test-Path $embedDist) {
    Remove-Item -Recurse -Force $embedDist
  }
  New-Item -ItemType Directory -Force $embedDist | Out-Null
  Copy-Item -Recurse -Force (Join-Path $Root "ui/dist/*") $embedDist
}

$commit = "unknown"
$git = Get-NativeCommand @("git.exe", "git")
if ($git -and (Test-Path (Join-Path $Root ".git"))) {
  $candidate = & $git -C $Root rev-parse --short=12 HEAD 2>$null
  if ($LASTEXITCODE -eq 0 -and $candidate) {
    $commit = $candidate.Trim()
  }
}
$buildDate = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$ldflags = "-s -w -X main.version=$Version -X main.commit=$commit -X main.date=$buildDate"

$dist = Join-Path $Root "dist"
if (Test-Path $dist) {
  Remove-Item -Recurse -Force $dist
}
New-Item -ItemType Directory -Force $dist | Out-Null

$targets = @(
  @{ GOOS = "linux"; GOARCH = "amd64"; Ext = "" },
  @{ GOOS = "linux"; GOARCH = "arm64"; Ext = "" },
  @{ GOOS = "darwin"; GOARCH = "amd64"; Ext = "" },
  @{ GOOS = "darwin"; GOARCH = "arm64"; Ext = "" },
  @{ GOOS = "windows"; GOARCH = "amd64"; Ext = ".exe" }
)

$oldGOOS = $env:GOOS
$oldGOARCH = $env:GOARCH
$oldCGO = $env:CGO_ENABLED
try {
  foreach ($target in $targets) {
    $name = "sigilbridge_${Version}_$($target.GOOS)_$($target.GOARCH)"
    $outDir = Join-Path $dist $name
    New-Item -ItemType Directory -Force $outDir | Out-Null

    $env:GOOS = $target.GOOS
    $env:GOARCH = $target.GOARCH
    $env:CGO_ENABLED = "0"
    $binary = Join-Path $outDir ("sigilbridge" + $target.Ext)

    Write-Host "building $name"
    & $go build -trimpath -ldflags "$ldflags" -o $binary ./cmd/sigilbridge
    if ($LASTEXITCODE -ne 0) {
      throw "go build failed for $name with exit code $LASTEXITCODE"
    }
    if (-not (Test-Path $binary) -or (Get-Item $binary).Length -lt 1024) {
      throw "release binary missing or unexpectedly small for $name"
    }

    Copy-Item (Join-Path $Root "README.md") (Join-Path $outDir "README.md")
    Copy-Item (Join-Path $Root "LICENSE") (Join-Path $outDir "LICENSE")
    Copy-Item (Join-Path $Root "examples/config.yaml") (Join-Path $outDir "config.example.yaml")
    Copy-Item (Join-Path $Root "examples/pools.yaml") (Join-Path $outDir "pools.example.yaml")

    & $tar -C $dist -czf (Join-Path $dist "$name.tar.gz") $name
    if ($LASTEXITCODE -ne 0) {
      throw "tar failed for $name with exit code $LASTEXITCODE"
    }
    Remove-Item -Recurse -Force $outDir
  }
} finally {
  $env:GOOS = $oldGOOS
  $env:GOARCH = $oldGOARCH
  $env:CGO_ENABLED = $oldCGO
}

$checksumLines = Get-ChildItem $dist -Filter "*.tar.gz" |
  Sort-Object Name |
  ForEach-Object {
    "$((Get-FileHash -Algorithm SHA256 $_.FullName).Hash.ToLowerInvariant())  $($_.Name)"
  }
[System.IO.File]::WriteAllText((Join-Path $dist "checksums.txt"), ($checksumLines -join "`n") + "`n", [System.Text.UTF8Encoding]::new($false))

if ($PublishDocker) {
  $docker = Get-NativeCommand @("docker.exe", "docker")
  if (-not $docker) {
    throw "docker is required when -PublishDocker is set."
  }
  $image = if ($env:DOCKER_IMAGE) { $env:DOCKER_IMAGE } else { "ghcr.io/sigilbridge/sigilbridge" }
  & $docker buildx build `
    --platform linux/amd64,linux/arm64 `
    --build-arg "VERSION=$Version" `
    --build-arg "COMMIT=$commit" `
    --build-arg "BUILD_DATE=$buildDate" `
    -t "${image}:${Version}" `
    -t "${image}:latest" `
    --push `
    -f (Join-Path $Root "deployments/docker/Dockerfile") `
    $Root
  if ($LASTEXITCODE -ne 0) {
    throw "docker publish failed with exit code $LASTEXITCODE"
  }
}

Write-Host "release artifacts written to $dist"
