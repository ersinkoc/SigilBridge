package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServesAssetsWithProductionHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/"+firstAsset(t), nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp := httptest.NewRecorder()

	Handler().ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := resp.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q", got)
	}
	if !strings.Contains(resp.Header().Get("Vary"), "Accept-Encoding") {
		t.Fatalf("Vary = %q", resp.Header().Get("Vary"))
	}
}

func TestHandlerServesSPAFallbackWithoutImmutableCache(t *testing.T) {
	resp := httptest.NewRecorder()

	Handler().ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/keys/missing", nil))

	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "SigilBridge") {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

func firstAsset(t *testing.T) string {
	t.Helper()
	entries, err := assets.ReadDir("dist/assets")
	if err != nil {
		t.Fatalf("ReadDir(dist/assets) error = %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			return entry.Name()
		}
	}
	t.Fatal("no admin UI asset found")
	return ""
}
