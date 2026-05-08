package observability

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestNewLoggerWritesRequiredFields(t *testing.T) {
	var buf bytes.Buffer
	logger, err := NewLogger(&buf, LoggerConfig{Level: "info", Format: "json"})
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	logger.Info(
		"request completed",
		slog.String(FieldRequestID, "01HX2B8QQ7H0ZZQJYB2QC9X4C3"),
		slog.String(FieldBridgeKeyID, "key_123"),
		slog.String(FieldPool, "sonnet"),
		slog.String(FieldUpstreamID, "anth-a"),
		slog.String(FieldProvider, "anthropic_api"),
		slog.String(FieldEvent, "request_completed"),
		slog.Int(FieldLatencyMS, 12),
	)

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("log line is not JSON: %v", err)
	}
	for _, field := range []string{FieldRequestID, FieldBridgeKeyID, FieldPool, FieldUpstreamID, FieldProvider, FieldEvent, FieldLatencyMS} {
		if _, ok := got[field]; !ok {
			t.Fatalf("missing log field %q in %#v", field, got)
		}
	}
}
