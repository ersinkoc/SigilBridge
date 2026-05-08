$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $Root

Remove-Item -Recurse -Force dist, ui/dist, ui/.lighthouseci, ui/test-results, .lighthouseci, data, backup, audit, examples/data, examples/backup, examples/audit -ErrorAction SilentlyContinue
Remove-Item -Force coverage.out -ErrorAction SilentlyContinue
Remove-Item -Force sigilbridge, sigilbridge.exe -ErrorAction SilentlyContinue

Remove-Item -Recurse -Force internal/admin/ui/dist -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force internal/admin/ui/dist/assets | Out-Null
$index = @'
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>SigilBridge Admin</title>
  </head>
  <body>
    <div id="root">SigilBridge admin UI assets have not been built yet.</div>
  </body>
</html>
'@
[System.IO.File]::WriteAllText(
  (Join-Path $Root "internal/admin/ui/dist/index.html"),
  $index,
  [System.Text.UTF8Encoding]::new($false)
)
"placeholder" | Set-Content -NoNewline -Encoding ascii internal/admin/ui/dist/assets/placeholder.txt

Write-Host "Cleaned generated artifacts."
