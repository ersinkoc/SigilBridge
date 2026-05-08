# SigilBridge Operator Runbook

## Purpose

This runbook covers day-to-day operation of a single SigilBridge instance: deployment, recovery, backups, vault handling, plugins, and common failure modes.

## Deployment Checklist

1. Create a dedicated OS user.
2. Create writable state directories, for example `/var/lib/sigilbridge` and `/var/log/sigilbridge`.
3. Generate a 32-byte base64 master key and store it in your secrets manager.
4. Write `config.yaml`, `pools.yaml`, and `admin_tokens.yaml`, or run `sigilbridge init --dir /etc/sigilbridge` and then edit the generated files for production.
5. Start SigilBridge with `SIGILBRIDGE_MASTER_KEY` set in the process environment.
6. Verify `/healthz`, `/readyz`, and `/metrics`.
7. Create the first bridge key through the admin UI or CLI.
8. Send a request through `/v1/chat/completions` or `/v1/messages`.

## Minimal Production Config

```yaml
server:
  bind: 127.0.0.1:8787
  max_concurrent_requests: 1024
  request_timeout_seconds: 600
  idle_timeout_seconds: 120
  shutdown_grace_seconds: 30

admin:
  bind: 127.0.0.1:8788
  tokens_file: admin_tokens.yaml
  ui_enabled: true

storage:
  path: /var/lib/sigilbridge/sigilbridge.db
  busy_timeout_ms: 5000
  cache_size_kb: 20000
  mmap_size_mb: 256
  backup:
    enabled: true
    interval_hours: 24
    retention_days: 14
    path: /var/lib/sigilbridge/backup

audit:
  enabled: true
  content_mode: none
  retention_days: 90
  rotate_compress_after_days: 7

vault:
  master_key_env: SIGILBRIDGE_MASTER_KEY

logging:
  level: info
  format: json

metrics:
  prometheus_enabled: true

pools_file: pools.yaml
```

## Health Checks

- `GET /healthz`: process is alive.
- `GET /readyz`: configured pools have at least one usable upstream.
- `GET /metrics`: Prometheus exposition.
- Admin health view: per-upstream state, latency, breaker state, and in-flight requests.

If `/healthz` passes but `/readyz` fails, inspect pool config, provider credentials, DNS, and upstream rate limits.

## Admin Surface

- Browser UI: `/admin/ui/`.
- Admin API: `/admin/v1/...`.
- Admin tokens can be sent as `Authorization: Bearer <admin-token>` for automation.
- Browser sessions use the `sigilbridge_admin` cookie with `HttpOnly` and `SameSite=Strict`.
- Cookie-authenticated write requests require a same-origin `Origin` or `Referer` header. This blocks cross-site form/fetch attempts while keeping bearer-token automation usable.
- Admin UI responses set clickjacking, content sniffing, referrer, permissions, and CSP headers. Static hashed assets are cacheable; HTML remains revalidatable.

## Backup And Restore

Back up SQLite with the built-in backup command or the storage package's `VACUUM INTO` flow:

```bash
sigilbridge backup \
  --config /etc/sigilbridge/config.yaml \
  --output /var/lib/sigilbridge/backup/sigilbridge-$(date +%F).db
```

Restore only while the service is stopped:

```bash
systemctl stop sigilbridge
sigilbridge restore \
  --config /etc/sigilbridge/config.yaml \
  --from /var/lib/sigilbridge/backup/sigilbridge-2026-05-07.db
systemctl start sigilbridge
```

Keep the master key with the backup set. Encrypted vault rows cannot be opened without the original key.

## Vault Key Rotation

Recommended rotation flow:

1. Stop request traffic or put the instance in maintenance.
2. Export or snapshot the database.
3. Start a controlled re-encryption job that reads each vault entry with the old key and writes it with the new key.
4. Update `SIGILBRIDGE_MASTER_KEY`.
5. Restart the bridge.
6. Verify OAuth/session credentials can be read.
7. Remove the old key from active secret stores after rollback risk has passed.

Do not rotate by deleting the database or changing the environment variable alone. Existing ciphertext requires the old key.

## Hot Reload

Use the admin API or UI reload action after changing hot-reloadable files:

```bash
curl -X POST http://127.0.0.1:8788/admin/v1/reload \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{}'
```

Pool, admin token, pricing, and most operational fields are reloadable. Listener binds, storage path, and master key require restart.

## Audit Maintenance

Manual audit rotation and pruning use the `audit/` directory beside the config file:

```bash
sigilbridge maintenance prune-audit --config /etc/sigilbridge/config.yaml
```

Files older than `audit.rotate_compress_after_days` are gzipped. Files older than `audit.retention_days` are deleted.

## Common Failures

| Symptom | Likely Cause | Resolution |
| --- | --- | --- |
| `server.bind is required` | Empty or malformed config | Validate `config.yaml` and keep known fields only. |
| `requires SIGILBRIDGE_MASTER_KEY` | OAuth, session, or `vault://` upstream without master key | Set the configured master key env var before startup. |
| 401 on ingress | Missing or invalid bridge key | Use `Authorization: Bearer sb_...` for OpenAI ingress or `x-api-key` for Anthropic ingress. |
| 403 on admin write | Cookie-authenticated request missing same-origin proof | Send the request from `/admin/ui/`, include a same-origin `Origin`/`Referer`, or use `Authorization: Bearer <admin-token>` for automation. |
| 402 budget exceeded | Key hard cap reached | Raise the key budget or route through a different key. |
| 429 rate limited | Key rate limit or upstream rate limit | Increase key limits, add upstream capacity, or wait for cooldown. |
| `/readyz` fails | No healthy upstream in a required pool | Probe upstreams, check credentials, and inspect breaker state. |
| OAuth refresh failed | Expired or revoked refresh token | Reconnect the credential from the OAuth setup flow. |
| CLI agent unavailable | Executable missing or not authenticated | Install the CLI, sign in with its native auth command, and probe again. |
| Plugin not loaded | Bad manifest, executable path, or handshake failure | Validate `plugin.yaml`, permissions, and plugin logs. |

## Plugin Install

1. Build the plugin binary for the host OS and architecture.
2. Create a plugin directory under the configured plugin root.
3. Place `plugin.yaml` and the binary in that directory.
4. Restart or reload plugin discovery.
5. Add an upstream with `provider` set to the plugin provider ID.

See [plugins.md](plugins.md) for the full development guide.

## Security Baseline

- Bind admin to localhost or a private network.
- Put TLS at a reverse proxy if the bridge is reachable over a network.
- Preserve `Host` and `X-Forwarded-Proto` when proxying admin traffic so same-origin checks match the public admin URL.
- Enable compression for text assets at the proxy if it terminates HTTP before SigilBridge.
- Run as an unprivileged user.
- Use file permissions `0700` for state and audit directories.
- Keep `SIGILBRIDGE_MASTER_KEY` out of config files and logs.
- Set budgets and rate limits on every production bridge key.
- Keep session adapters disabled unless the operator has accepted provider ToS risk.
