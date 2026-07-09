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
}

// NewServer wires the routes and applies explicit timeouts — cheap
// insurance against slow-client resource exhaustion on a public endpoint.
func NewServer(cfg ServerConfig, h *Handlers, log *slog.Logger) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/plans", h.GetPlans)
	mux.HandleFunc("GET /healthz", h.Healthz)

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
