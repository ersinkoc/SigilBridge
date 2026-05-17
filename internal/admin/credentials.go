package admin

import (
	"net/http"
	"strings"
)

func (s *Server) credentials(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		id, err := credentialIDFromRequest(r)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.services.Credentials.Delete(r.Context(), id); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	out, err := s.services.Credentials.List(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) oauthBootstrap(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider string `json:"provider"`
		Name     string `json:"name"`
		Mode     string `json:"mode"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Provider) == "" {
		writeErr(w, http.StatusBadRequest, "provider is required")
		return
	}
	out, err := s.services.Credentials.OAuthBootstrap(r.Context(), req.Provider, req.Name, req.Mode)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) oauthProviders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		out, err := s.services.Credentials.OAuthProvidersRaw(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodPut:
		var req struct {
			Body string `json:"body"`
		}
		if err := decode(r, &req); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := s.services.Credentials.OAuthProvidersSave(r.Context(), req.Body)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) apiKeyCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req APIKeyCredentialRequest
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	out, err := s.services.Credentials.APIKeyCreate(r.Context(), req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) sessionCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req SessionCredentialRequest
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	out, err := s.services.Credentials.SessionCreate(r.Context(), req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) oauthRefresh(w http.ResponseWriter, r *http.Request) {
	id, err := credentialIDFromRequest(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	out, err := s.services.Credentials.OAuthRefresh(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) oauthRevoke(w http.ResponseWriter, r *http.Request) {
	id, err := credentialIDFromRequest(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	out, err := s.services.Credentials.OAuthRevoke(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) oauthCallback(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	_, err := s.services.Credentials.OAuthCallback(r.Context(), values.Get("state"), values.Get("code"), values.Get("error"))
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("<!doctype html><title>SigilBridge OAuth failed</title><h1>OAuth failed</h1><p>Authentication failed. Please try again or contact your administrator.</p>"))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("<!doctype html><title>SigilBridge OAuth complete</title><h1>OAuth credential stored</h1><p>You can close this tab.</p>"))
}

func (s *Server) cliStatus(w http.ResponseWriter, r *http.Request) {
	out, err := s.services.Credentials.CLIStatus(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) cliDetect(w http.ResponseWriter, r *http.Request) {
	out, err := s.services.Credentials.CLIDetect(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) cliEnable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req CLIEnableRequest
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	out, err := s.services.Credentials.CLIEnable(r.Context(), req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) providerCatalog(w http.ResponseWriter, r *http.Request) {
	out, err := s.services.Credentials.ProviderCatalog(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func credentialIDFromRequest(r *http.Request) (string, error) {
	if id := r.URL.Query().Get("id"); id != "" {
		return id, nil
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := decode(r, &req); err != nil {
		return "", err
	}
	if req.ID == "" {
		return "", credentialRequestError("credential id is required")
	}
	return req.ID, nil
}

type credentialRequestError string

func (e credentialRequestError) Error() string { return string(e) }
