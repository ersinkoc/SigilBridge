package admin

import "net/http"

func (s *Server) reload(w http.ResponseWriter, r *http.Request) {
	out, err := s.services.Reload.Reload(r.Context())
	if err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	if !out.OK && len(out.RestartRequiredFields) > 0 {
		writeJSON(w, http.StatusConflict, out)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) eventsStream(w http.ResponseWriter, r *http.Request) {
	if s.services.Events == nil {
		writeErr(w, http.StatusServiceUnavailable, "event bus not configured")
		return
	}
	s.services.Events.ServeHTTP(w, r)
}
