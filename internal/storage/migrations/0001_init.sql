-- +goose Up
CREATE TABLE bridge_keys (
    id                TEXT PRIMARY KEY,
    hash              TEXT NOT NULL UNIQUE,
    name              TEXT NOT NULL,
    created_at        DATETIME NOT NULL,
    created_by        TEXT,
    last_used_at      DATETIME,
    revoked_at        DATETIME,
    scopes_json       TEXT NOT NULL,
    budgets_json      TEXT NOT NULL,
    rate_limits_json  TEXT NOT NULL,
    metadata_json     TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_bridge_keys_hash_active
  ON bridge_keys(hash) WHERE revoked_at IS NULL;

CREATE TABLE sessions (
    id                  TEXT PRIMARY KEY,
    provider            TEXT NOT NULL,
    created_at          DATETIME NOT NULL,
    last_refreshed_at   DATETIME,
    expires_at          DATETIME,
    nonce               BLOB NOT NULL,
    ciphertext          BLOB NOT NULL,
    metadata_json       TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE budget_counters (
    key_id   TEXT NOT NULL,
    period   TEXT NOT NULL,
    bucket   TEXT NOT NULL,
    cents    INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (key_id, period, bucket),
    FOREIGN KEY (key_id) REFERENCES bridge_keys(id) ON DELETE CASCADE
);

CREATE TABLE ratelimit_buckets (
    key_id   TEXT NOT NULL,
    metric   TEXT NOT NULL,
    bucket   INTEGER NOT NULL,
    value    INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (key_id, metric, bucket),
    FOREIGN KEY (key_id) REFERENCES bridge_keys(id) ON DELETE CASCADE
);

CREATE INDEX idx_ratelimit_bucket ON ratelimit_buckets(bucket);

CREATE TABLE audit_index (
    request_id      TEXT PRIMARY KEY,
    ts              DATETIME NOT NULL,
    bridge_key_id   TEXT,
    pool_name       TEXT,
    upstream_id     TEXT,
    status          TEXT NOT NULL,
    cost_cents      INTEGER NOT NULL DEFAULT 0,
    file_path       TEXT NOT NULL,
    file_offset     INTEGER NOT NULL,
    file_length     INTEGER NOT NULL
);

CREATE INDEX idx_audit_ts   ON audit_index(ts);
CREATE INDEX idx_audit_key  ON audit_index(bridge_key_id, ts);
CREATE INDEX idx_audit_pool ON audit_index(pool_name, ts);

CREATE TABLE cooldowns (
    upstream_id            TEXT PRIMARY KEY,
    pool_name              TEXT NOT NULL,
    state                  TEXT NOT NULL,
    consecutive_failures   INTEGER NOT NULL DEFAULT 0,
    last_error             TEXT,
    last_error_at          DATETIME,
    last_success_at        DATETIME,
    cooldown_until         DATETIME,
    circuit_open_until     DATETIME,
    updated_at             DATETIME NOT NULL
);

CREATE TABLE events (
    id            TEXT PRIMARY KEY,
    ts            DATETIME NOT NULL,
    type          TEXT NOT NULL,
    severity      TEXT NOT NULL,
    payload_json  TEXT NOT NULL,
    actor         TEXT
);

CREATE INDEX idx_events_ts   ON events(ts);
CREATE INDEX idx_events_type ON events(type, ts);

-- +goose Down
DROP INDEX IF EXISTS idx_events_type;
DROP INDEX IF EXISTS idx_events_ts;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS cooldowns;
DROP INDEX IF EXISTS idx_audit_pool;
DROP INDEX IF EXISTS idx_audit_key;
DROP INDEX IF EXISTS idx_audit_ts;
DROP TABLE IF EXISTS audit_index;
DROP INDEX IF EXISTS idx_ratelimit_bucket;
DROP TABLE IF EXISTS ratelimit_buckets;
DROP TABLE IF EXISTS budget_counters;
DROP TABLE IF EXISTS sessions;
DROP INDEX IF EXISTS idx_bridge_keys_hash_active;
DROP TABLE IF EXISTS bridge_keys;

