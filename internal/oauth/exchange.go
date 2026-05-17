package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/httpclient"
)

type ExchangeRequest struct {
	TokenURL     string
	ClientID     string
	Code         string
	CodeVerifier string
	RedirectURI  string
	RefreshToken string
}

func ExchangeCode(ctx context.Context, client HTTPClient, req ExchangeRequest) (Token, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", req.Code)
	form.Set("code_verifier", req.CodeVerifier)
	form.Set("client_id", req.ClientID)
	form.Set("redirect_uri", req.RedirectURI)
	return tokenPost(ctx, client, req.TokenURL, form)
}

func RefreshToken(ctx context.Context, client HTTPClient, req ExchangeRequest) (Token, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", req.RefreshToken)
	form.Set("client_id", req.ClientID)
	return tokenPost(ctx, client, req.TokenURL, form)
}

func RevokeToken(ctx context.Context, client HTTPClient, revokeURL, clientID, token string) error {
	if revokeURL == "" {
		return nil
	}
	form := url.Values{"token": {token}, "client_id": {clientID}}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, revokeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("revoke endpoint returned %d", resp.StatusCode)
	}
	return nil
}

func tokenPost(ctx context.Context, client HTTPClient, tokenURL string, form url.Values) (Token, error) {
	if client == nil {
		client = httpclient.Default()
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(httpReq)
	if err != nil {
		return Token{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Token{}, fmt.Errorf("read token response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return Token{}, parseTokenError(raw, resp.StatusCode)
	}
	return decodeToken(raw)
}

func decodeToken(raw []byte) (Token, error) {
	var payload struct {
		AccessToken  string          `json:"access_token"`
		RefreshToken string          `json:"refresh_token"`
		TokenType    string          `json:"token_type"`
		Scope        string          `json:"scope"`
		ExpiresIn    json.RawMessage `json:"expires_in"`
		ExpiresAt    string          `json:"expires_at"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return Token{}, fmt.Errorf("parse token response: %w", err)
	}
	token := Token{AccessToken: payload.AccessToken, RefreshToken: payload.RefreshToken, TokenType: payload.TokenType, Scope: payload.Scope, Raw: append([]byte(nil), raw...)}
	if payload.AccessToken == "" {
		return Token{}, &OAuthError{Code: "missing_access_token"}
	}
	if len(payload.ExpiresIn) > 0 {
		if seconds, ok := parseExpiresIn(payload.ExpiresIn); ok {
			token.ExpiresAt = time.Now().UTC().Add(time.Duration(seconds) * time.Second)
		}
	}
	if payload.ExpiresAt != "" {
		if parsed, err := time.Parse(time.RFC3339, payload.ExpiresAt); err == nil {
			token.ExpiresAt = parsed.UTC()
		}
	}
	return token, nil
}

func parseExpiresIn(raw json.RawMessage) (int64, bool) {
	var number int64
	if json.Unmarshal(raw, &number) == nil {
		return number, true
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		parsed, err := strconv.ParseInt(text, 10, 64)
		return parsed, err == nil
	}
	return 0, false
}

func parseTokenError(raw []byte, status int) error {
	var payload struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(raw, &payload)
	if payload.Error == "" {
		payload.Error = fmt.Sprintf("http_%d", status)
		payload.ErrorDescription = "token request failed"
	}
	return &OAuthError{Code: payload.Error, Description: payload.ErrorDescription, Temporary: status >= 500 || status == http.StatusTooManyRequests || payload.Error == "authorization_pending" || payload.Error == "slow_down"}
}
