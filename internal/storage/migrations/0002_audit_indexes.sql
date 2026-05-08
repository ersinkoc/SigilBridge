-- +goose Up
CREATE INDEX IF NOT EXISTS idx_audit_status_ts ON audit_index(status, ts);

-- +goose Down
DROP INDEX IF EXISTS idx_audit_status_ts;

