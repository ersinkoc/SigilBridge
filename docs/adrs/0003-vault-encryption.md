# ADR-0003: Vault Encryption

## Status

Accepted

## Context

OAuth tokens and session credentials need at-rest protection even if the database file is copied. Bridge keys are hashed and do not need decryption, but vault records do.

## Decision

Encrypt vault values with AES-256-GCM using a 32-byte base64 master key from `SIGILBRIDGE_MASTER_KEY` or the configured env var. Use associated data to bind ciphertext to record identity.

## Consequences

- Database theft does not expose credential plaintext without the master key.
- The master key must be backed up and injected securely.
- Rotation requires a controlled re-encryption flow.
- Go cannot guarantee perfect memory zeroization, but best-effort locking and wiping are still useful.

## Alternatives Considered

- Store credentials in OS keychains: less portable for server deployments.
- Use cloud KMS only: useful later, but not self-hosted by default.
- Encrypt the whole SQLite file: simpler operationally, but weaker record-level binding and harder portability.
