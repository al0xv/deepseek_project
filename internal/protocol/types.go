// Package protocol defines the messages exchanged between the terminal
// client and the gateway over HTTP and Server-Sent Events.
package protocol

// SessionState describes the lifecycle of a session.
type SessionState string

// Session states.
const (
	StateWaiting   SessionState = "WAITING"
	StateApproved  SessionState = "APPROVED"
	StateActive    SessionState = "ACTIVE"
	StateDestroyed SessionState = "DESTROYED"
)

// SessionCreateRequest is POSTed to /v1/sessions.
type SessionCreateRequest struct {
	ClientName string `json:"client_name,omitempty"`
}

// SessionCreateResponse is returned by POST /v1/sessions.
// PairingToken is included so the client can render the QR code; it is
// single-use and expires after ExpiresIn seconds.
type SessionCreateResponse struct {
	SessionID    string `json:"session_id"`
	PairingToken string `json:"pairing_token"`
	PairingCode  string `json:"pairing_code"`
	GatewayURL   string `json:"gateway_url"`
	ExpiresIn    int    `json:"expires_in"`
}

// SessionStatusResponse is returned by GET /v1/sessions/{id}.
// Model/ThinkingEnabled/ReasoningEffort are the canonical effective settings
// of an approved session (validated by the gateway), for display by the
// terminal client.
type SessionStatusResponse struct {
	SessionID       string       `json:"session_id"`
	State           SessionState `json:"state"`
	Model           string       `json:"model,omitempty"`
	ThinkingEnabled *bool        `json:"thinking_enabled,omitempty"`
	ReasoningEffort string       `json:"reasoning_effort,omitempty"`
}

// PromptRequest is POSTed to /v1/sessions/{id}/prompt.
type PromptRequest struct {
	Content string `json:"content"`
}

// TokenEvent is streamed for every text delta.
type TokenEvent struct {
	Delta string `json:"delta"`
}

// DoneEvent signals the end of a streamed completion.
type DoneEvent struct {
	FinishReason string `json:"finish_reason"` // "stop" | "length" | "cancelled"
}

// ErrorEvent signals an error during streaming.
type ErrorEvent struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error codes used in HTTP error bodies and SSE error events.
const (
	ErrUpstream        = "upstream_error"
	ErrTimeout         = "generation_timeout"
	ErrPairingPending  = "pairing_pending"
	ErrPairingExpired  = "pairing_expired"
	ErrAlreadyPaired   = "already_approved"
	ErrNotFound        = "not_found"
	ErrTooManySessions = "too_many_sessions"
	ErrNotApproved     = "not_approved"
	ErrBadRequest      = "bad_request"
	ErrUnauthorized    = "unauthorized"
	ErrInvalidSettings = "invalid_settings"
	// ErrNotImplemented marks features that exist in the wire protocol but are
	// not implemented yet. It is returned instead of silently ignoring the
	// request so nobody mistakes a stub for a working feature.
	ErrNotImplemented = "not_implemented"
)

// ErrorBody is the JSON body of non-200 HTTP responses.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ApproveRequest is POSTed to /v1/pair. Either PairingToken (from the QR
// code) or PairingCode (typed fallback) must be set. APIKey is reserved for
// the future iOS controller (MVP3). Model/Thinking/ReasoningEffort are
// optional; empty fields fall back to the gateway defaults.
type ApproveRequest struct {
	PairingToken    string `json:"pairing_token,omitempty"`
	PairingCode     string `json:"pairing_code,omitempty"`
	APIKey          string `json:"api_key,omitempty"`
	Model           string `json:"model,omitempty"`
	Thinking        *bool  `json:"thinking,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

// ApproveResponse is returned by POST /v1/pair.
// ControlToken is a temporary controller capability valid for the lifetime of
// the session: it is memory-only, never shown to the terminal client and is
// invalidated once the session is destroyed.
type ApproveResponse struct {
	SessionID    string       `json:"session_id"`
	State        SessionState `json:"state"`
	ControlToken string       `json:"control_token,omitempty"`
}

// NormalizePairCode strips everything except digits: "472 913" -> "472913".
func NormalizePairCode(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			out = append(out, s[i])
		}
	}
	return string(out)
}

// FormatPairCode renders "472913" as "472 913".
func FormatPairCode(code string) string {
	if len(code) != 6 {
		return code
	}
	return code[:3] + " " + code[3:]
}
