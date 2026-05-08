package ingress

import (
	"io"
	"net/http"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/ir"
)

func (s *Server) anthropicMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	keyID, ok := s.authorize(w, r)
	if !ok {
		return
	}
	raw, _ := io.ReadAll(r.Body)
	req, err := ir.NormalizeAnthropicRequest(raw, keyID, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.dispatch(w, r, req, ir.DenormalizeAnthropicResponse, anthropicStream)
}
