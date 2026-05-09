# Release Validation

Use this checklist before cutting a public SigilBridge release. It separates local validation from checks that require GitHub, a fresh host, or real provider accounts.

## Local Validation

From the repository root:

```powershell
.\test.ps1 -Race -SkipUI
pnpm --dir ui run lint
pnpm --dir ui run test
pnpm --dir ui run build
pnpm --dir ui run test:e2e
pnpm --dir ui run lhci
.\release.ps1 -Version v0.0.0-local -SkipUIBuild
```

Each release archive should include the binary, `README.md`, `LICENSE`, `config.example.yaml`, `pools.example.yaml`, and `oauth_providers.example.yaml`.

`test:e2e` and `lhci` both build or serve `ui/dist`; run them sequentially, not in parallel with `build.ps1` or `release.ps1`.

On Windows, `test.ps1 -Race` uses Docker automatically when `gcc` is not available.

For a local cross-compile smoke:

```powershell
$targets = @(
  @("linux", "amd64"),
  @("linux", "arm64"),
  @("darwin", "amd64"),
  @("darwin", "arm64"),
  @("windows", "amd64")
)
foreach ($target in $targets) {
  $env:GOOS = $target[0]
  $env:GOARCH = $target[1]
  $env:CGO_ENABLED = "0"
  $suffix = if ($env:GOOS -eq "windows") { ".exe" } else { "" }
  go build -trimpath -o "dist/ci-local/sigilbridge-$($env:GOOS)-$($env:GOARCH)$suffix" ./cmd/sigilbridge
}
Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
```

## GitHub CI

Push a branch and confirm the CI workflow is green for:

- Linux amd64
- Linux arm64
- macOS amd64
- macOS arm64
- Windows amd64

The release workflow should be checked on a temporary tag before the final signed tag.

## Real OAuth E2E

Prerequisites:

- A valid `SIGILBRIDGE_MASTER_KEY`.
- Provider OAuth metadata in `~/.sigilbridge/oauth_providers.yaml`.
- An active subscription account for the selected provider.

Flow:

1. Run `sigilbridge oauth add <provider-registry-id> --config <config.yaml> --name release-smoke`.
2. Configure a pool upstream with `provider: claude_oauth` and `credential: vault://oauth/<provider-registry-id>/release-smoke`.
3. Start `sigilbridge serve --config <config.yaml>`.
4. Create a bridge key.
5. Send an OpenAI-compatible request to `/v1/chat/completions`.
6. Confirm the response, audit record, and budget counter are present.

## Real CLI-Agent E2E

Prerequisites:

- Install the target CLI as the same OS user that runs SigilBridge.
- Complete the CLI's native login flow.
- Verify a manual CLI prompt works outside SigilBridge.

Flow:

1. Configure a pool upstream with `provider: claude_code_cli` and an absolute `executable` path.
2. Start `sigilbridge serve --config <config.yaml>`.
3. Send a streaming request through `/v1/chat/completions`.
4. Confirm streamed content, process health, stderr capture, audit record, and idle shutdown.

## Fresh-Host Deployment

Docker:

```bash
docker build -f deployments/docker/Dockerfile -t sigilbridge:release-smoke .
docker run --rm sigilbridge:release-smoke version
```

For a full container smoke on a local host:

```powershell
.\scripts\docker-smoke.ps1
```

For a deployed admin URL behind a reverse proxy:

```powershell
$env:SIGILBRIDGE_ADMIN_TOKEN = "<admin-token>"
.\scripts\admin-proxy-preflight.ps1 -AdminUrl "https://bridge.example.com"
```

This validates browser-style cookie login and a same-origin admin write through the public URL. It should pass before announcing a proxy-backed deployment as ready.

systemd:

1. Create the `sigilbridge` user and group.
2. Install the binary at `/usr/local/bin/sigilbridge`.
3. Copy `deployments/systemd/sigilbridge.service` to `/etc/systemd/system/`.
4. Place config under `/etc/sigilbridge/` and state under `/var/lib/sigilbridge/`.
5. Run `systemctl daemon-reload && systemctl start sigilbridge`.
6. Confirm `systemctl status sigilbridge`, `/healthz`, `/readyz`, and `/metrics`.

## Signed Tag

```bash
git tag -s v1.0.0 -m "SigilBridge v1.0.0"
git push origin v1.0.0
```

After the release workflow completes, verify checksums, SLSA provenance, and Docker image pull/run.
