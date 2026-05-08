param(
  [switch]$VerboseOutput
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location $Root

if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
  throw "git is required"
}

$visiblePaths = @(
  git ls-files --cached --others --exclude-standard
) | Where-Object { $_ -and -not $_.StartsWith("dist/") -and -not $_.StartsWith("artifacts/") }

$forbiddenPathPatterns = @(
  "^(config|pools|oauth_providers|admin_tokens)\.ya?ml$",
  "^\.env(\.|$)",
  "^\.sigilbridge/",
  "^(data|backup|audit)/",
  "^artifacts/",
  "(^|/).*\.(db|db-wal|db-shm|pem|key|p12|pfx)$"
)

$pathFindings = New-Object System.Collections.Generic.List[string]
foreach ($path in $visiblePaths) {
  $normalized = $path -replace "\\", "/"
  foreach ($pattern in $forbiddenPathPatterns) {
    if ($normalized -match $pattern) {
      $pathFindings.Add($normalized)
      break
    }
  }
}

$secretPatterns = @(
  @{ Name = "Generic sk token"; Pattern = "\bsk-[A-Za-z0-9_-]{20,}\b" },
  @{ Name = "GitHub classic token"; Pattern = "\bghp_[A-Za-z0-9_]{20,}\b" },
  @{ Name = "GitHub fine-grained token"; Pattern = "\bgithub_pat_[A-Za-z0-9_]{20,}\b" },
  @{ Name = "Slack token"; Pattern = "\bxox[baprs]-[A-Za-z0-9-]{20,}\b" },
  @{ Name = "AWS access key"; Pattern = "\bAKIA[0-9A-Z]{16}\b" },
  @{ Name = "Private key block"; Pattern = "-----BEGIN (RSA |EC |OPENSSH |)PRIVATE KEY-----" },
  @{ Name = "Assigned secret value"; Pattern = "(?i)(password|passwd|client_secret|api[_-]?key|secret|access_token|refresh_token|admin_token)\s*[:=]\s*['""]?[A-Za-z0-9_./+=-]{16,}" }
)

$contentFindings = New-Object System.Collections.Generic.List[string]
foreach ($path in $visiblePaths) {
  $fullPath = Join-Path $Root $path
  if (-not (Test-Path -LiteralPath $fullPath -PathType Leaf)) {
    continue
  }
  $item = Get-Item -LiteralPath $fullPath
  if ($item.Length -gt 2MB) {
    continue
  }
  foreach ($candidate in $secretPatterns) {
    $matches = Select-String -LiteralPath $fullPath -Pattern $candidate.Pattern -AllMatches -ErrorAction SilentlyContinue
    foreach ($match in $matches) {
      $contentFindings.Add("${path}:$($match.LineNumber): $($candidate.Name)")
    }
  }
}

if ($pathFindings.Count -gt 0 -or $contentFindings.Count -gt 0) {
  Write-Error "Sensitive files or secret-looking values are visible to git."
  if ($pathFindings.Count -gt 0) {
    Write-Host "Forbidden git-visible paths:"
    $pathFindings | Sort-Object -Unique | ForEach-Object { Write-Host "  $_" }
  }
  if ($contentFindings.Count -gt 0) {
    Write-Host "Secret-looking content:"
    $contentFindings | Sort-Object -Unique | ForEach-Object { Write-Host "  $_" }
  }
  exit 1
}

if ($VerboseOutput) {
  Write-Host "Checked $($visiblePaths.Count) git-visible paths."
}
Write-Host "No sensitive local files or secret-looking values are visible to git."
