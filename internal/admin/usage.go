package admin

import "net/http"

func (s *Server) usage(w http.ResponseWriter, r *http.Request) {
	out, err := s.services.Budgets.Usage(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
