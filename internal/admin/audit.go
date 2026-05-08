package admin

import "net/http"

func (s *Server) audit(w http.ResponseWriter, r *http.Request) {
	out, err := s.services.Audit.Query(r.Context(), r.URL.Query())
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
