package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeviceAuthorizationPendingThenSuccess(t *testing.T) {
	var polls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/device":
			_, _ = w.Write([]byte(`{"device_code":"device-1","user_code":"USER-1","verification_uri":"https://example.test/device","interval":1,"expires_in":60}`))
		case "/token":
			polls++
			if polls == 1 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"access-device","refresh_token":"refresh-device"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	provider := Provider{ID: "stub", ClientID: "client-1", DeviceAuthURL: server.URL + "/device", TokenURL: server.URL + "/token"}
	auth, err := StartDeviceAuthorization(context.Background(), server.Client(), provider, nil)
	if err != nil {
		t.Fatalf("StartDeviceAuthorization() error = %v", err)
	}
	token, err := PollDeviceAuthorization(context.Background(), server.Client(), provider, auth)
	if err != nil {
		t.Fatalf("PollDeviceAuthorization() error = %v", err)
	}
	if token.AccessToken != "access-device" || polls != 2 {
		t.Fatalf("token=%#v polls=%d", token, polls)
	}
}
