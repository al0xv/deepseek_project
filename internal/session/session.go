// Package session manages the in-memory lifecycle of gateway sessions.
package session

import (
	"errors"
	"sync"
	"time"

	"deepseek-terminal/internal/provider"
	"deepseek-terminal/internal/protocol"
)

// Session state aliases.
const (
	StateWaiting   = protocol.StateWaiting
	StateApproved  = protocol.StateApproved
	StateActive    = protocol.StateActive
	StateDestroyed = protocol.StateDestroyed
)

// Errors returned by the manager.
var (
	ErrNotFound        = errors.New("session not found")
	ErrMaxSessions     = errors.New("too many sessions")
	ErrPairingExpired  = errors.New("pairing expired")
	ErrAlreadyApproved = errors.New("session already approved")
	ErrNotApproved     = errors.New("session is not approved")
	ErrNotActive       = errors.New("no generation in flight")
)

// Session is a single terminal session. It lives entirely in memory and is
// destroyed explicitly; nothing is ever persisted.
type Session struct {
	ID           string
	State        protocol.SessionState
	PairToken    string
	PairCode     string
	PairExpiry   time.Time
	IdleDeadline time.Time
	History      []provider.Message
	APIKey       string // only populated by the iOS controller (MVP3)

	mu        sync.Mutex
	genCancel func()
}
