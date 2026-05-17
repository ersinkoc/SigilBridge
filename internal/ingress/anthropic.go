package ingress

import (
	"io"
	"net/http"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/ir"
)

const maxIngressBodyBytes int64 = 1 << 20 // 1MB

func (s *Server) anthropicMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	keyID, ok := s.authorize(w, r)
	if !ok {
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxIngressBodyBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	if int64(len(raw)) > maxIngressBodyBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
		return
	}
	req, err := ir.NormalizeAnthropicRequest(raw, keyID, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.dispatch(w, r, req, ir.DenormalizeAnthropicResponse, anthropicStream)
}
