package admin

import "net/http"

func (s *Server) endpoints(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.services.Endpoints == nil {
		writeErr(w, http.StatusServiceUnavailable, "endpoint info is not configured")
		return
	}
	out, err := s.services.Endpoints.Info(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
