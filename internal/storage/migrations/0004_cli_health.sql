-- +goose Up
CREATE TABLE cli_health (
    upstream_id       TEXT PRIMARY KEY,
    command           TEXT NOT NULL,
    pid               INTEGER,
    status            TEXT NOT NULL,
    last_started_at   DATETIME,
    last_seen_at      DATETIME,
    stderr_tail       TEXT NOT NULL DEFAULT '',
    metadata_json     TEXT NOT NULL DEFAULT '{}'
);

-- +goose Down
DROP TABLE IF EXISTS cli_health;

