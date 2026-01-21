// Package frontend provides SPA serving utilities for Go web applications.
// It supports both development mode (proxying to Vite dev server) and
// production mode (serving embedded static files).
package frontend

import (
	"embed"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

const (
	// DefaultDevHost is the default Vite dev server address.
	DefaultDevHost = "localhost:5173"
)

// Config holds the configuration for frontend serving.
type Config struct {
	// DevHost is the address of the Vite dev server (default: localhost:5173).
	DevHost string

	// DistPath is the path to the embedded dist directory (e.g., "frontend/dist").
	DistPath string

	// IndexFile is the name of the index file (default: "index.html").
	IndexFile string
}

// DefaultConfig returns the default frontend configuration.
func DefaultConfig() *Config {
	return &Config{
		DevHost:   DefaultDevHost,
		DistPath:  "frontend/dist",
		IndexFile: "index.html",
	}
}

// Setup configures frontend serving on the given router.
// In development mode (detected via IsDev()), it proxies requests to the Vite dev server.
// In production mode, it serves static files from the embedded filesystem.
func Setup(router *mux.Router, frontend embed.FS, distPath string) {
	SetupWithConfig(router, frontend, &Config{
		DevHost:   DefaultDevHost,
		DistPath:  distPath,
		IndexFile: "index.html",
	})
}

// SetupWithConfig configures frontend serving with custom configuration.
func SetupWithConfig(router *mux.Router, frontend embed.FS, cfg *Config) {
	if cfg.DevHost == "" {
		cfg.DevHost = DefaultDevHost
	}
	if cfg.IndexFile == "" {
		cfg.IndexFile = "index.html"
	}

	if IsDev() {
		log.Info().
			Str("devHost", cfg.DevHost).
			Msg("Dev mode detected. Frontend is being proxied to Vite dev server")

		proxy := httputil.NewSingleHostReverseProxy(&url.URL{
			Scheme: "http",
			Host:   cfg.DevHost,
		})
		router.PathPrefix("/").Handler(proxy)
	} else {
		log.Info().
			Str("distPath", cfg.DistPath).
			Msg("Production mode detected. Serving frontend from embedded filesystem")

		handler := NewSPAHandler(frontend, cfg.DistPath, cfg.IndexFile)
		router.PathPrefix("/").Handler(handler)
	}
}

// IsDev returns true when the application is running via `go run`.
// It detects this by checking if the executable path contains "go-build",
// which is the temporary directory used by `go run`.
func IsDev() bool {
	ex, err := os.Executable()
	if err != nil {
		return false
	}
	return strings.Contains(filepath.Dir(ex), "go-build")
}

// SPAHandler serves a Single Page Application from an embedded filesystem.
// It serves static files when they exist and falls back to index.html for
// client-side routing.
type SPAHandler struct {
	fs        embed.FS
	distPath  string
	indexFile string
}

// NewSPAHandler creates a new SPA handler.
func NewSPAHandler(frontend embed.FS, distPath, indexFile string) *SPAHandler {
	return &SPAHandler{
		fs:        frontend,
		distPath:  distPath,
		indexFile: indexFile,
	}
}

// ServeHTTP implements http.Handler for serving the SPA.
func (h *SPAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Get the absolute path to prevent directory traversal
	path, err := filepath.Abs(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Prepend the path with the static directory path
	path = filepath.Join(h.distPath, path)

	// Check if the file exists
	_, err = h.fs.Open(path)
	if os.IsNotExist(err) {
		// File does not exist, serve index.html for SPA routing
		h.serveIndex(w, r)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Serve the static file
	sub, err := fs.Sub(h.fs, h.distPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.FileServer(http.FS(sub)).ServeHTTP(w, r)
}

// serveIndex serves the index.html file.
func (h *SPAHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	indexPath := filepath.Join(h.distPath, h.indexFile)
	index, err := h.fs.ReadFile(indexPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(index)
}
