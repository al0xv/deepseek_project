package gateway

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"deepseek-terminal/internal/protocol"
	"deepseek-terminal/internal/provider"
	"deepseek-terminal/internal/session"
)

func (g *Gateway) handleCreate(w http.ResponseWriter, r *http.Request) {
	if !g.limiter.Allow() {
		writeError(w, http.StatusTooManyRequests, protocol.ErrTooManySessions, "too many session creations, slow down")
		return
	}
	var req protocol.SessionCreateRequest
	_ = decodeJSON(r, &req, 1024)

	s, err := g.mgr.Create()
	if err != nil {
		if errors.Is(err, session.ErrMaxSessions) {
			writeError(w, http.StatusTooManyRequests, protocol.ErrTooManySessions, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, protocol.ErrUpstream, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, protocol.SessionCreateResponse{
		SessionID:    s.ID,
		PairingToken: s.PairToken,
		PairingCode:  protocol.FormatPairCode(s.PairCode),
		GatewayURL:   g.gatewayURL,
		ExpiresIn:    int(g.mgr.Cfg().PairTimeout / time.Second),
	})
}

func (g *Gateway) handleApprove(w http.ResponseWriter, r *http.Request) {
	var req protocol.ApproveRequest
	if err := decodeJSON(r, &req, 4096); err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrBadRequest, "invalid body")
		return
	}

	settings, err := provider.ParseSettings(provider.Model(g.model), req.Model, req.Thinking, req.ReasoningEffort)
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidSettings, err.Error())
		return
	}

	// STUB (Phase 3.4): the wire protocol accepts api_key, but secure delivery
	// of a session-scoped DeepSeek API key from the iOS controller to this
	// gateway is NOT implemented yet. Reject it explicitly instead of silently
	// ignoring it, and do not consume the pairing credential in the process.
	if req.APIKey != "" {
		writeError(w, http.StatusNotImplemented, protocol.ErrNotImplemented,
			"iPhone API-key delivery is a stub (Phase 3.4): set DEEPSEEK_API_KEY on the gateway instead")
		return
	}

	var s *session.Session
	switch {
	case req.PairingToken != "":
		s, err = g.mgr.ApproveByTokenWithSettings(req.PairingToken, settings)
	case req.PairingCode != "":
		s, err = g.mgr.ApproveByCodeWithSettings(protocol.NormalizePairCode(req.PairingCode), settings)
	default:
		writeError(w, http.StatusBadRequest, protocol.ErrBadRequest, "pairing_token or pairing_code required")
		return
	}

	switch {
	case errors.Is(err, session.ErrNotFound):
		writeError(w, http.StatusNotFound, protocol.ErrNotFound, "unknown pairing credential")
		return
	case errors.Is(err, session.ErrPairingExpired):
		writeError(w, http.StatusGone, protocol.ErrPairingExpired, "pairing expired, create a new session")
		return
	case errors.Is(err, session.ErrAlreadyApproved):
		writeError(w, http.StatusConflict, protocol.ErrAlreadyPaired, "session already approved")
		return
	case errors.Is(err, session.ErrInvalidSettings):
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidSettings, err.Error())
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, protocol.ErrUpstream, err.Error())
		return
	}

	tok, err := g.mgr.ControlToken(s.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, protocol.ErrUpstream, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, protocol.ApproveResponse{
		SessionID:    s.ID,
		State:        s.State,
		ControlToken: tok,
	})
}

func (g *Gateway) handleStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	state, err := g.mgr.Status(id)
	if err != nil {
		writeError(w, http.StatusNotFound, protocol.ErrNotFound, "session not found")
		return
	}
	resp := protocol.SessionStatusResponse{SessionID: id, State: state}
	if settings, err := g.mgr.SessionSettings(id); err == nil && settings.Model != "" {
		resp.Model = string(settings.Model)
		t := settings.ThinkingEnabled
		resp.ThinkingEnabled = &t
		resp.ReasoningEffort = string(settings.ReasoningEffort)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (g *Gateway) handleClose(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := g.mgr.Get(id); !ok {
		writeError(w, http.StatusNotFound, protocol.ErrNotFound, "session not found")
		return
	}
	g.mgr.Destroy(id)
	w.WriteHeader(http.StatusNoContent)
}

// handleEnd is the controller endpoint: it destroys a session only when the
// request proves ownership via the control token issued at approval.
func (g *Gateway) handleEnd(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" {
		writeError(w, http.StatusUnauthorized, protocol.ErrUnauthorized, "missing control token")
		return
	}
	err := g.mgr.EndByControlToken(id, token)
	switch {
	case errors.Is(err, session.ErrNotFound):
		writeError(w, http.StatusNotFound, protocol.ErrNotFound, "session not found")
	case errors.Is(err, session.ErrUnauthorized):
		writeError(w, http.StatusForbidden, protocol.ErrUnauthorized, "invalid control token")
	case err != nil:
		writeError(w, http.StatusInternalServerError, protocol.ErrUpstream, err.Error())
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// bearerToken extracts the token from an "Authorization: Bearer <token>"
// header value. It returns "" when missing or not a Bearer scheme.
func bearerToken(header string) string {
	const prefix = "Bearer "
	if len(header) > len(prefix) && strings.EqualFold(header[:len(prefix)], prefix) {
		return strings.TrimSpace(header[len(prefix):])
	}
	return ""
}

func (g *Gateway) handleCancel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := g.mgr.CancelGeneration(id); err != nil {
		writeError(w, http.StatusNotFound, protocol.ErrNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (g *Gateway) handlePrompt(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req protocol.PromptRequest
	if err := decodeJSON(r, &req, g.maxPrompt); err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrBadRequest, "invalid body")
		return
	}
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, protocol.ErrBadRequest, "empty content")
		return
	}

	history, err := g.mgr.PreparePrompt(id, req.Content)
	if err != nil {
		switch {
		case errors.Is(err, session.ErrNotFound):
			writeError(w, http.StatusNotFound, protocol.ErrNotFound, "session not found")
		case errors.Is(err, session.ErrNotApproved):
			state, _ := g.mgr.Status(id)
			code := protocol.ErrNotApproved
			msg := "session is not approved"
			if state == protocol.StateWaiting {
				code = protocol.ErrPairingPending
				msg = "session is waiting for approval"
			}
			writeError(w, http.StatusForbidden, code, msg)
		default:
			writeError(w, http.StatusInternalServerError, protocol.ErrUpstream, err.Error())
		}
		return
	}

	g.streamPrompt(w, r, id, history)
}

// streamPrompt writes the SSE response for one prompt. It owns the generation
// context: a separate /cancel request or a client disconnect both cancel it.
func (g *Gateway) streamPrompt(w http.ResponseWriter, r *http.Request, id string, history []provider.Message) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		g.mgr.FailPrompt(id)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), g.genTimeout)
	defer cancel()
	_ = g.mgr.SetGenCancel(id, cancel)

	// Cancel the generation when the client disconnects mid-stream.
	watchDone := make(chan struct{})
	go func() {
		select {
		case <-r.Context().Done():
			cancel()
		case <-watchDone:
		}
	}()

	onDelta := func(delta string) error {
		if err := protocol.WriteSSE(w, "token", protocol.TokenEvent{Delta: delta}); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	settings, _ := g.mgr.SessionSettings(id)
	result, err := g.provider.StreamCompletion(ctx, provider.CompletionRequest{
		Settings: settings,
		Messages: history,
	}, onDelta)
	close(watchDone)

	switch {
	case r.Context().Err() != nil:
		// Client disconnected mid-stream: destroy the session.
		g.mgr.Destroy(id)
	case err != nil:
		g.mgr.FailPrompt(id)
		_ = protocol.WriteSSE(w, "error", protocol.ErrorEvent{Code: protocol.ErrUpstream, Message: err.Error()})
		flusher.Flush()
	case ctx.Err() == context.DeadlineExceeded:
		g.mgr.FailPrompt(id)
		_ = protocol.WriteSSE(w, "error", protocol.ErrorEvent{Code: protocol.ErrTimeout, Message: "generation timed out"})
		flusher.Flush()
	case ctx.Err() == context.Canceled:
		// User pressed cancel: keep whatever was generated.
		g.mgr.FinishPrompt(id, result.FullText, provider.FinishCanceled)
		_ = protocol.WriteSSE(w, "done", protocol.DoneEvent{FinishReason: provider.FinishCanceled})
		flusher.Flush()
	default:
		g.mgr.FinishPrompt(id, result.FullText, result.FinishReason)
		_ = protocol.WriteSSE(w, "done", protocol.DoneEvent{FinishReason: result.FinishReason})
		flusher.Flush()
	}
}
