package ingress

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/ir"
)

func (s *Server) openAIChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	keyID, ok := s.authorize(w, r)
	if !ok {
		return
	}
	raw, _ := io.ReadAll(r.Body)
	req, err := ir.NormalizeOAIRequest(raw, keyID, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.dispatch(w, r, req, ir.DenormalizeOAIResponse, oaiStream)
}

func (s *Server) models(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authorize(w, r); !ok {
		return
	}
	modelIDs := s.modelIDs
	if provider, ok := s.dispatcher.(ModelProvider); ok {
		modelIDs = provider.Models()
	}
	data := make([]any, 0, len(modelIDs))
	for _, model := range modelIDs {
		data = append(data, map[string]any{"id": model, "object": "model", "owned_by": "sigilbridge"})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": data})
}
