package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/httpclient"
)

type DeviceAuthorization struct {
	DeviceCode              string    `json:"device_code"`
	UserCode                string    `json:"user_code"`
	VerificationURI         string    `json:"verification_uri"`
	VerificationURIComplete string    `json:"verification_uri_complete,omitempty"`
	Interval                int       `json:"interval"`
	ExpiresAt               time.Time `json:"expires_at"`
}

func StartDeviceAuthorization(ctx context.Context, client HTTPClient, provider Provider, scopes []string) (DeviceAuthorization, error) {
	if provider.DeviceAuthURL == "" {
		return DeviceAuthorization{}, fmt.Errorf("provider %q has no device authorization endpoint", provider.ID)
	}
	if client == nil {
		client = httpclient.Default()
	}
	form := url.Values{"client_id": {provider.ClientID}}
	if len(scopes) == 0 {
		scopes = provider.DefaultScopes
	}
	if len(scopes) > 0 {
		form.Set("scope", strings.Join(scopes, " "))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.DeviceAuthURL, strings.NewReader(form.Encode()))
	if err != nil {
		return DeviceAuthorization{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return DeviceAuthorization{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return DeviceAuthorization{}, parseTokenError(raw, resp.StatusCode)
	}
	var out struct {
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationURI         string `json:"verification_uri"`
		VerificationURIComplete string `json:"verification_uri_complete"`
		Interval                int    `json:"interval"`
		ExpiresIn               int    `json:"expires_in"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return DeviceAuthorization{}, fmt.Errorf("parse device authorization response: %w", err)
	}
	if out.DeviceCode == "" || out.UserCode == "" {
		return DeviceAuthorization{}, &OAuthError{Code: "invalid_device_authorization_response"}
	}
	if out.Interval <= 0 {
		out.Interval = 5
	}
	if out.ExpiresIn <= 0 {
		out.ExpiresIn = 900
	}
	return DeviceAuthorization{DeviceCode: out.DeviceCode, UserCode: out.UserCode, VerificationURI: out.VerificationURI, VerificationURIComplete: out.VerificationURIComplete, Interval: out.Interval, ExpiresAt: time.Now().UTC().Add(time.Duration(out.ExpiresIn) * time.Second)}, nil
}

func PollDeviceAuthorization(ctx context.Context, client HTTPClient, provider Provider, auth DeviceAuthorization) (Token, error) {
	if client == nil {
		client = httpclient.Default()
	}
	interval := time.Duration(auth.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	for {
		token, err := pollDeviceOnce(ctx, client, provider, auth.DeviceCode)
		if err == nil {
			return token, nil
		}
		oauthErr, ok := err.(*OAuthError)
		if !ok {
			return Token{}, err
		}
		switch oauthErr.Code {
		case "authorization_pending":
		case "slow_down":
			interval += 5 * time.Second
		case "access_denied", "expired_token":
			return Token{}, err
		default:
			if !oauthErr.Temporary {
				return Token{}, err
			}
		}
		timer := time.NewTimer(interval)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return Token{}, ctx.Err()
		}
	}
}

func pollDeviceOnce(ctx context.Context, client HTTPClient, provider Provider, deviceCode string) (Token, error) {
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	form.Set("device_code", deviceCode)
	form.Set("client_id", provider.ClientID)
	return tokenPost(ctx, client, provider.TokenURL, form)
}
