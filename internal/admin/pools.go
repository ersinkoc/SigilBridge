package admin

import "net/http"

func (s *Server) pools(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		pools, err := s.services.Pools.List(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, pools)
	case http.MethodPost:
		var pool PoolDTO
		if err := decode(r, &pool); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := s.services.Pools.Upsert(r.Context(), pool)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) poolDetail(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "/admin/v1/pools/")
	if len(id) > 6 && id[len(id)-6:] == "/probe" {
		id = id[:len(id)-6]
		result, err := s.services.Pools.Probe(r.Context(), id)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	if r.Method == http.MethodDelete {
		if err := s.services.Pools.Delete(r.Context(), id); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}
