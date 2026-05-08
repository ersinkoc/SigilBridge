# OAuth Setup Guide

## Overview

OAuth adapters let SigilBridge route through subscription-backed accounts without storing raw passwords or browser cookies. The bridge supports Authorization Code with PKCE for desktop bootstrap and Device Authorization Grant for headless environments.

Tokens are stored in the encrypted vault. Set `SIGILBRIDGE_MASTER_KEY` before adding any OAuth credential.

## Configure Provider Registry

SigilBridge loads OAuth provider metadata from `~/.sigilbridge/oauth_providers.yaml` or the path configured by `oauth.providers_file`. It intentionally does not ship fake usable providers or public client IDs.

```yaml
providers:
  - id: my_provider
    display_name: My Provider
    auth_url: https://auth.your-provider.com/oauth/authorize
    token_url: https://auth.your-provider.com/oauth/token
    device_auth_url: https://auth.your-provider.com/oauth/device/code
    revoke_url: https://auth.your-provider.com/oauth/revoke
    client_id: your-registered-client-id
    default_scopes:
      - offline_access
```

Use provider documentation or enterprise SSO metadata for the real URLs and scopes. Provider public clients can rotate, so keep this file operator-owned.

## Browser PKCE Flow

```bash
sigilbridge oauth add claude_max --config /etc/sigilbridge/config.yaml --name personal
```

Expected flow:

1. SigilBridge generates a verifier, challenge, and state.
2. A localhost callback listener starts on `127.0.0.1:0`.
3. The system browser opens the provider authorization URL.
4. The user signs in and consents.
5. The provider redirects to the localhost callback.
6. SigilBridge exchanges the code and seals the token in the vault.

## Device Flow

Use device flow when the host has no browser:

```bash
sigilbridge oauth add claude_max --config /etc/sigilbridge/config.yaml --name headless --device
```

The command displays a verification URL and user code. Complete the authorization on another device; the bridge polls until the token arrives or the device code expires.

## Pool Configuration

```yaml
pools:
  - name: claude
    strategy: priority_first
    upstreams:
      - id: claude-max-personal
        provider: claude_oauth
        priority: 1
        weight: 1
        config:
          credential: vault://oauth/claude_max/personal
          model: claude-sonnet-4-5
```

## Provider Notes

### Claude Max / Pro

- Preferred adapter: `claude_oauth`.
- Use PKCE where available; use device grant for headless hosts.
- If the provider requires organization selection, store the selected organization in upstream config.
- Legacy fallback: `claude_web`, disabled by default and subject to ToS risk.

### GitHub Copilot

- Preferred adapter: `copilot_oauth`.
- Device flow is usually the most convenient option for servers.
- Confirm the account has an active Copilot entitlement before adding the credential.

### Gemini Advanced

- Preferred adapter: `gemini_oauth`.
- Enterprise Google environments may require an operator-owned OAuth client.
- Ensure the requested scopes allow model invocation and refresh token issuance.

### Cursor Pro

- Preferred adapter: `cursor_oauth`.
- Marked experimental because provider behavior can change.
- Keep a fallback API-key or CLI upstream in the same pool.

## Operations

- Refresh worker checks credentials on a timer and refreshes tokens before expiry.
- Failed refreshes emit admin events and should be handled before the credential expires.
- Revoke credentials that are no longer needed:

```bash
sigilbridge oauth revoke vault://oauth/claude_max/personal --config /etc/sigilbridge/config.yaml
```

## Troubleshooting

| Symptom | Resolution |
| --- | --- |
| State mismatch | Restart the flow; do not reuse old authorization URLs. |
| `invalid_grant` | Code expired, verifier mismatch, or refresh token revoked. Reconnect. |
| Device code expired | Start a new device flow. |
| No refresh token returned | Add `offline_access` or provider-specific offline scope if supported. |
| Vault open failed | Verify the same master key is used across restarts. |
