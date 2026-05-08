package oauth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExchangeCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form := string(body)
		for _, want := range []string{"grant_type=authorization_code", "code=code-1", "code_verifier=verifier-1", "client_id=client-1"} {
			if !strings.Contains(form, want) {
				t.Fatalf("form missing %q: %s", want, form)
			}
		}
		_, _ = w.Write([]byte(`{"access_token":"access-1","refresh_token":"refresh-1","token_type":"Bearer","expires_in":3600}`))
	}))
	defer server.Close()

	token, err := ExchangeCode(context.Background(), server.Client(), ExchangeRequest{TokenURL: server.URL, ClientID: "client-1", Code: "code-1", CodeVerifier: "verifier-1", RedirectURI: "http://127.0.0.1/cb"})
	if err != nil {
		t.Fatalf("ExchangeCode() error = %v", err)
	}
	if token.AccessToken != "access-1" || token.RefreshToken != "refresh-1" || token.ExpiresAt.IsZero() {
		t.Fatalf("token = %#v", token)
	}
}

func TestTokenError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"bad code"}`))
	}))
	defer server.Close()
	_, err := ExchangeCode(context.Background(), server.Client(), ExchangeRequest{TokenURL: server.URL})
	oauthErr, ok := err.(*OAuthError)
	if !ok || oauthErr.Code != "invalid_grant" || oauthErr.Temporary {
		t.Fatalf("err = %#v", err)
	}
}

func TestTokenParsingVariantsAndRevokeNoop(t *testing.T) {
	token, err := decodeToken([]byte(`{"access_token":"access","expires_in":"42","expires_at":"2026-01-02T03:04:05Z"}`))
	if err != nil {
		t.Fatalf("decodeToken() error = %v", err)
	}
	if token.AccessToken != "access" || token.ExpiresAt.IsZero() {
		t.Fatalf("token = %#v", token)
	}
	if _, err := decodeToken([]byte(`{"refresh_token":"missing-access"}`)); err == nil {
		t.Fatal("decodeToken() missing access token error = nil")
	}
	if err := RevokeToken(context.Background(), nil, "", "client", "token"); err != nil {
		t.Fatalf("RevokeToken(empty URL) error = %v", err)
	}
	err = parseTokenError([]byte(`temporary outage`), http.StatusServiceUnavailable)
	oauthErr, ok := err.(*OAuthError)
	if !ok || oauthErr.Code != "http_503" || !oauthErr.Temporary {
		t.Fatalf("parseTokenError() = %#v", err)
	}
}
