package observability

import (
	"context"

	"github.com/oklog/ulid/v2"
)

type requestIDKey struct{}

func NewRequestID() string {
	return ulid.Make().String()
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

func RequestID(ctx context.Context) (string, bool) {
	requestID, ok := ctx.Value(requestIDKey{}).(string)
	return requestID, ok && requestID != ""
}
