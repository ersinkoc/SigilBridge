# SigilBridge Server Runner
# Usage: Double-click this file or run: .\run.ps1

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$EnvFile = Join-Path $ProjectRoot ".env"
$ConfigFile = Join-Path $ProjectRoot ".sigilbridge\config.yaml"

# Load .env file
if (Test-Path $EnvFile) {
    Get-Content $EnvFile | ForEach-Object {
        if ($_ -match "^([^=]+)=(.*)$") {
            [System.Environment]::SetEnvironmentVariable($matches[1], $matches[2], "Process")
        }
    }
    Write-Host "[OK] .env loaded" -ForegroundColor Green
} else {
    Write-Host "[ERROR] .env not found at: $EnvFile" -ForegroundColor Red
    exit 1
}

# Check required env vars
if (-not $env:SIGILBRIDGE_MASTER_KEY) {
    Write-Host "[ERROR] SIGILBRIDGE_MASTER_KEY not set" -ForegroundColor Red
    exit 1
}

# Kill any existing server on ports
$ports = @(8187, 8188)
foreach ($port in $ports) {
    $proc = Get-NetTCPConnection -LocalPort $port -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($proc) {
        Stop-Process -Id $proc.OwningProcess -Force -ErrorAction SilentlyContinue
        Write-Host "[OK] Killed existing process on port $port" -ForegroundColor Yellow
    }
}
Start-Sleep -Milliseconds 500

# Run server
Write-Host ""
Write-Host "Starting SigilBridge..." -ForegroundColor Cyan
Write-Host "API:     http://127.0.0.1:8187" -ForegroundColor Gray
Write-Host "Admin:   http://127.0.0.1:8188" -ForegroundColor Gray
Write-Host ""

cd $ProjectRoot
go run .\cmd\sigilbridge\... serve --config $ConfigFile