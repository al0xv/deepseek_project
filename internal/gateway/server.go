// Package gateway implements the HTTP server that exposes sessions to
// terminal clients and (later) the iOS controller.
package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"deepseek-terminal/internal/provider"
	"deepseek-terminal/internal/protocol"
	"deepseek-terminal/internal/session"
)

// Config configures the gateway.
type Config struct {
	Provider   provider.Provider
	Model      string
	Manager    *session.Manager
	GenTimeout time.Duration
	GatewayURL string
	// MaxPromptBytes limits the size of a prompt request body.
	MaxPromptBytes int64
}

// Gateway holds the dependencies of the HTTP handlers.
type Gateway struct {
	provider   provider.Provider
	model      string
	mgr        *session.Manager
	genTimeout time.Duration
	gatewayURL string
	maxPrompt  int64
	limiter    *createLimiter
}

// New returns a ready-to-serve Gateway.
func New(cfg Config) *Gateway {
	if cfg.Model == "" {
		cfg.Model = "deepseek-chat"
	}
	if cfg.GenTimeout <= 0 {
		cfg.GenTimeout = 60 * time.Second
	}
	if cfg.MaxPromptBytes <= 0 {
		cfg.MaxPromptBytes = 64 * 1024
	}
	return &Gateway{
		provider:   cfg.Provider,
		model:      cfg.Model,
		mgr:        cfg.Manager,
		genTimeout: cfg.GenTimeout,
		gatewayURL: cfg.GatewayURL,
		maxPrompt:  cfg.MaxPromptBytes,
		limiter:    newCreateLimiter(20, time.Minute),
	}
}

// Handler returns the http.Handler with all routes.
func (g *Gateway) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/sessions", g.handleCreate)
	mux.HandleFunc("POST /v1/pair", g.handleApprove)
	mux.HandleFunc("GET /v1/sessions/{id}", g.handleStatus)
	mux.HandleFunc("POST /v1/sessions/{id}/prompt", g.handlePrompt)
	mux.HandleFunc("POST /v1/sessions/{id}/cancel", g.handleCancel)
	mux.HandleFunc("POST /v1/sessions/{id}/close", g.handleClose)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, protocol.ErrorBody{Code: code, Message: msg})
}

func decodeJSON(r *http.Request, v any, maxBytes int64) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBytes))
	return dec.Decode(v)
}
