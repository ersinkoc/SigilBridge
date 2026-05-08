package admin

import "net/http"

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	out, err := s.services.Health.Detail(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
