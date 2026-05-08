package observability

import (
	"context"
	"testing"
)

func TestRequestIDRoundTrip(t *testing.T) {
	id := NewRequestID()
	if len(id) != 26 {
		t.Fatalf("ULID length = %d, want 26", len(id))
	}

	got, ok := RequestID(WithRequestID(context.Background(), id))
	if !ok || got != id {
		t.Fatalf("RequestID() = %q, %v; want %q, true", got, ok, id)
	}
}
