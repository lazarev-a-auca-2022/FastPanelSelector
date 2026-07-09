package httpapi

import (
	"log/slog"
	"net/http"
	"time"
)

type ServerConfig struct {
	Addr              string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	// StaticDir, if non-empty, is served at "/" — the frontend selector's
	// static files (index.html, app.js, style.css, ...). Left empty for an
	// API-only deployment.
	StaticDir string
}

// NewServer wires the routes and applies explicit timeouts — cheap
// insurance against slow-client resource exhaustion on a public endpoint.
func NewServer(cfg ServerConfig, h *Handlers, log *slog.Logger) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/plans", h.GetPlans)
	mux.HandleFunc("GET /healthz", h.Healthz)
	if cfg.StaticDir != "" {
		// Registered last/least-specific: Go's ServeMux prefers the more
		// specific "/api/plans" and "/healthz" patterns over this "/"
		// catch-all, so this only ever serves the frontend's own files.
		mux.Handle("/", http.FileServer(http.Dir(cfg.StaticDir)))
	}

	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelError),
	}
}
