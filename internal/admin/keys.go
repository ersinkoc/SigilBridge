package admin

import "net/http"

func (s *Server) keys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		keys, err := s.services.Keys.List(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, keys)
	case http.MethodPost:
		var req CreateKeyRequest
		if err := decode(r, &req); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		key, err := s.services.Keys.Create(r.Context(), req)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, key)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) keyDetail(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "/admin/v1/keys/")
	switch r.Method {
	case http.MethodGet:
		key, err := s.services.Keys.Get(r.Context(), id)
		if err != nil {
			writeErr(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, key)
	case http.MethodPatch:
		patch := map[string]any{}
		if err := decode(r, &patch); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		key, err := s.services.Keys.Patch(r.Context(), id, patch)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, key)
	case http.MethodDelete:
		if err := s.services.Keys.Delete(r.Context(), id); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
