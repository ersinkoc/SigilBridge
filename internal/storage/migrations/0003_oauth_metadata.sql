-- +goose Up
CREATE TABLE oauth_metadata (
    id             TEXT PRIMARY KEY,
    provider       TEXT NOT NULL,
    subject        TEXT,
    scopes_json    TEXT NOT NULL DEFAULT '[]',
    created_at     DATETIME NOT NULL,
    updated_at     DATETIME NOT NULL,
    metadata_json  TEXT NOT NULL DEFAULT '{}',
    FOREIGN KEY (id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE INDEX idx_oauth_metadata_provider ON oauth_metadata(provider);

-- +goose Down
DROP INDEX IF EXISTS idx_oauth_metadata_provider;
DROP TABLE IF EXISTS oauth_metadata;

