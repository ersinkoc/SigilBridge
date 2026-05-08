package ui

import (
	"bytes"
	"compress/gzip"
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed all:dist
var assets embed.FS

func Handler() http.Handler {
	sub, err := fs.Sub(assets, "dist")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "admin UI assets unavailable", http.StatusInternalServerError)
		})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "." || name == "/" {
			serveAsset(w, r, sub, "index.html")
			return
		}
		if _, err := fs.Stat(sub, name); err == nil {
			serveAsset(w, r, sub, name)
			return
		}
		if strings.HasPrefix(name, "assets/") {
			http.NotFound(w, r)
			return
		}
		serveAsset(w, r, sub, "index.html")
	})
}

func serveAsset(w http.ResponseWriter, r *http.Request, filesystem fs.FS, name string) {
	info, err := fs.Stat(filesystem, name)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	raw, err := fs.ReadFile(filesystem, name)
	if err != nil {
		http.Error(w, "admin UI asset unavailable", http.StatusInternalServerError)
		return
	}
	setAssetHeaders(w.Header(), name)
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	if shouldGzip(r, name) {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		w.WriteHeader(http.StatusOK)
		gz := gzip.NewWriter(w)
		_, _ = gz.Write(raw)
		_ = gz.Close()
		return
	}
	http.ServeContent(w, r, path.Base(name), time.Time{}, bytes.NewReader(raw))
}

func setAssetHeaders(header http.Header, name string) {
	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		header.Set("Content-Type", contentType)
	}
	if strings.HasPrefix(name, "assets/") {
		header.Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	header.Set("Cache-Control", "no-cache")
}

func shouldGzip(r *http.Request, name string) bool {
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		return false
	}
	switch path.Ext(name) {
	case ".css", ".html", ".js", ".json", ".svg":
		return true
	default:
		return false
	}
}
