package admin

import "net/http"

func (s *Server) chatTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.services.Chat == nil {
		writeErr(w, http.StatusServiceUnavailable, "chat tester is not configured")
		return
	}
	var req ChatTestRequest
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	out, err := s.services.Chat.Test(r.Context(), req)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
