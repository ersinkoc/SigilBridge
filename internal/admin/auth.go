package admin

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

type adminSubjectKey struct{}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	session, err := s.services.Auth.Login(r.Context(), req.Token)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, err.Error())
		return
	}
	http.SetCookie(w, sessionCookie(session, r.TLS != nil))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	// #nosec G124 -- sessionCookie sets Secure based on the current transport for local HTTP compatibility.
	cookie := sessionCookie("", r.TLS != nil)
	cookie.MaxAge = -1
	http.SetCookie(w, cookie)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) authenticate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subject, err := s.services.Auth.Verify(r.Context(), r)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if !validAdminWriteOrigin(r) {
			writeErr(w, http.StatusForbidden, "forbidden")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), adminSubjectKey{}, subject)))
	}
}

func validAdminWriteOrigin(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	if r.Header.Get("Authorization") != "" {
		return true
	}
	if _, err := r.Cookie("sigilbridge_admin"); err != nil {
		return true
	}
	expectedScheme, expectedHost := requestOrigin(r)
	for _, header := range []string{"Origin", "Referer"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value == "" {
			continue
		}
		if originMatches(value, expectedScheme, expectedHost) {
			return true
		}
		return false
	}
	return false
}

func requestOrigin(r *http.Request) (string, string) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwardedProto := firstForwardedValue(r.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = strings.ToLower(forwardedProto)
	}
	return scheme, strings.ToLower(r.Host)
}

func originMatches(value, expectedScheme, expectedHost string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Scheme, expectedScheme) && strings.EqualFold(parsed.Host, expectedHost)
}

func firstForwardedValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.IndexByte(value, ','); idx >= 0 {
		value = value[:idx]
	}
	return strings.TrimSpace(value)
}
