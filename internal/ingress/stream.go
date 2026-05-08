package ingress

import (
	"net/http"

	"github.com/sigilbridge/sigilbridge/internal/ir"
)

func writeSSE(w http.ResponseWriter, raw []byte) {
	w.Header().Set("Content-Type", "text/event-stream")
	_, _ = w.Write(raw)
}

func oaiStream(resp ir.Response, events <-chan ir.Event) ([]byte, error) {
	return ir.EventsToOAIStream(resp.ID, resp.UpstreamModel, events)
}

func anthropicStream(resp ir.Response, events <-chan ir.Event) ([]byte, error) {
	return ir.EventsToAnthropicStream(resp.ID, resp.UpstreamModel, events)
}
