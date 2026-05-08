package repos

import (
	"database/sql"
	"fmt"
	"time"
)

func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func nullTime(value time.Time) sql.NullString {
	if value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: value.UTC().Format(time.RFC3339Nano), Valid: true}
}

func scanNullString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func scanNullTime(value sql.NullString) (time.Time, error) {
	if !value.Valid || value.String == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", value.String, err)
	}
	return parsed, nil
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}
