package observability

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

const (
	FieldRequestID   = "request_id"
	FieldBridgeKeyID = "bridge_key_id"
	FieldPool        = "pool"
	FieldUpstreamID  = "upstream_id"
	FieldProvider    = "provider"
	FieldEvent       = "event"
	FieldLatencyMS   = "latency_ms"
)

type LoggerConfig struct {
	Level  string
	Format string
}

func NewLogger(w io.Writer, cfg LoggerConfig) (*slog.Logger, error) {
	level, err := ParseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	opts := &slog.HandlerOptions{Level: level}
	format := strings.ToLower(strings.TrimSpace(cfg.Format))
	if format == "" || format == "json" {
		return slog.New(slog.NewJSONHandler(w, opts)), nil
	}
	if format == "text" {
		return slog.New(slog.NewTextHandler(w, opts)), nil
	}
	return nil, fmt.Errorf("unsupported log format %q", cfg.Format)
}

func ParseLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unsupported log level %q", raw)
	}
}
