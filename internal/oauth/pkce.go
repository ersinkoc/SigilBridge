package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"
)

type PKCE struct {
	Verifier  string
	Challenge string
	State     string
	Method    string
}

func NewPKCE() (PKCE, error) {
	verifier, err := randomURLString(64)
	if err != nil {
		return PKCE{}, err
	}
	state, err := randomURLString(32)
	if err != nil {
		return PKCE{}, err
	}
	sum := sha256.Sum256([]byte(verifier))
	return PKCE{Verifier: verifier, Challenge: base64.RawURLEncoding.EncodeToString(sum[:]), State: state, Method: "S256"}, nil
}

func randomURLString(n int) (string, error) {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

type CallbackResult struct {
	Code  string
	State string
	Error string
}

type CallbackServer struct {
	RedirectURI string
	server      *http.Server
	results     chan CallbackResult
}

func StartCallbackServer(ctx context.Context, expectedState string) (*CallbackServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	results := make(chan CallbackResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		result := CallbackResult{Code: r.URL.Query().Get("code"), State: r.URL.Query().Get("state"), Error: r.URL.Query().Get("error")}
		if result.State != expectedState {
			http.Error(w, "invalid state", http.StatusBadRequest)
			results <- CallbackResult{Error: "invalid_state", State: result.State}
			return
		}
		fmt.Fprintln(w, "SigilBridge OAuth authorization complete. You can close this tab.")
		results <- result
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	callback := &CallbackServer{RedirectURI: "http://" + listener.Addr().String() + "/", server: server, results: results}
	go func() {
		_ = server.Serve(listener)
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	return callback, nil
}

func (s *CallbackServer) Wait(ctx context.Context) (CallbackResult, error) {
	select {
	case result := <-s.results:
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)
		if result.Error != "" {
			return result, &OAuthError{Code: result.Error}
		}
		if result.Code == "" {
			return result, &OAuthError{Code: "missing_code"}
		}
		return result, nil
	case <-ctx.Done():
		return CallbackResult{}, ctx.Err()
	}
}

func OpenBrowser(rawURL string) error {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("browser URL must use http or https")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// #nosec G204 -- rawURL is validated as an http(s) URL and passed as a single argument without a shell.
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	case "darwin":
		// #nosec G204 -- rawURL is validated as an http(s) URL and passed as a single argument without a shell.
		cmd = exec.Command("open", rawURL)
	default:
		// #nosec G204 -- rawURL is validated as an http(s) URL and passed as a single argument without a shell.
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}
