package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHandlerServesAssetsWithProductionHeaders(t *testing.T) {
	filesystem := fstest.MapFS{
		"assets/app.js": {Data: []byte("console.log('sigilbridge');")},
	}
	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp := httptest.NewRecorder()

	serveAsset(resp, req, filesystem, "assets/app.js")

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
	filesystem := fstest.MapFS{
		"index.html": {Data: []byte("<!doctype html><title>SigilBridge</title>")},
	}
	req := httptest.NewRequest(http.MethodGet, "/keys/missing", nil)
	resp := httptest.NewRecorder()

	serveAsset(resp, req, filesystem, "index.html")

	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "SigilBridge") {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q", got)
	}
}
