package session

import (
	"context"
	"crypto/subtle"
	"sync"
	"time"

	"deepseek-terminal/internal/crypto"
	"deepseek-terminal/internal/provider"
	"deepseek-terminal/internal/protocol"
)

// Clock abstracts time so tests can control it.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// Config configures a Manager.
type Config struct {
	Clock       Clock
	PairTimeout time.Duration
	IdleTimeout time.Duration
	GenTimeout  time.Duration
	MaxSessions int
}

// Manager owns the in-memory session map. All methods are safe for
// concurrent use. No data is ever persisted.
type Manager struct {
	mu         sync.Mutex
	sessions   map[string]*Session
	byPairCode map[string]*Session
	cfg        Config
}

// New returns a Manager with sensible defaults for zeroed fields.
func New(cfg Config) *Manager {
	if cfg.Clock == nil {
		cfg.Clock = realClock{}
	}
	if cfg.PairTimeout <= 0 {
		cfg.PairTimeout = 120 * time.Second
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = 5 * time.Minute
	}
	if cfg.GenTimeout <= 0 {
		cfg.GenTimeout = 60 * time.Second
	}
	if cfg.MaxSessions <= 0 {
		cfg.MaxSessions = 8
	}
	return &Manager{
		sessions:   make(map[string]*Session),
		byPairCode: make(map[string]*Session),
		cfg:        cfg,
	}
}

// Cfg returns the resolved configuration.
func (m *Manager) Cfg() Config { return m.cfg }

// Create makes a new WAITING session with a pairing token and code.
func (m *Manager) Create() (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.sessions) >= m.cfg.MaxSessions {
		return nil, ErrMaxSessions
	}
	id, err := crypto.NewID()
	if err != nil {
		return nil, err
	}
	token, err := crypto.NewToken()
	if err != nil {
		return nil, err
	}
	code, err := crypto.NewPairCode()
	if err != nil {
		return nil, err
	}
	now := m.cfg.Clock.Now()
	s := &Session{
		ID:           id,
		State:        protocol.StateWaiting,
		PairToken:    token,
		PairCode:     code,
		PairExpiry:   now.Add(m.cfg.PairTimeout),
		IdleDeadline: now.Add(m.cfg.IdleTimeout),
	}
	m.sessions[id] = s
	m.byPairCode[code] = s
	return s, nil
}

// SessionHistory returns a copy of the session's conversation history.
// It returns nil for unknown sessions.
func (m *Manager) SessionHistory(id string) []provider.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]provider.Message, len(s.History))
	copy(out, s.History)
	return out
}

// Get returns the session with id.
func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	return s, ok
}

// Status returns the current state of the session.
func (m *Manager) Status(id string) (protocol.SessionState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return "", ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.State, nil
}

// ApproveByToken approves a session using the long pairing token from the QR.
func (m *Manager) ApproveByToken(token string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var s *Session
	for _, cand := range m.sessions {
		cand.mu.Lock()
		matches := cand.PairToken == token
		cand.mu.Unlock()
		if matches {
			s = cand
			break
		}
	}
	if s == nil {
		return nil, ErrNotFound
	}
	return m.approve(s)
}

// ApproveByCode approves a session using the 6-digit pairing code.
func (m *Manager) ApproveByCode(code string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.byPairCode[code]
	if s == nil {
		return nil, ErrNotFound
	}
	return m.approve(s)
}

func (m *Manager) approve(s *Session) (*Session, error) {
	now := m.cfg.Clock.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if now.After(s.PairExpiry) {
		return nil, ErrPairingExpired
	}
	if s.State != protocol.StateWaiting {
		return nil, ErrAlreadyApproved
	}
	tok, err := crypto.NewToken()
	if err != nil {
		return nil, err
	}
	s.ControlToken = tok
	s.State = protocol.StateApproved
	return s, nil
}

// ControlToken returns the control token issued to the approving controller.
func (m *Manager) ControlToken(id string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return "", ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ControlToken, nil
}

// EndByControlToken destroys a session if token matches the control token
// issued at approval. The token is a controller capability: it is memory-only
// and becomes invalid as soon as the session is destroyed.
func (m *Manager) EndByControlToken(id, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return ErrNotFound
	}
	s.mu.Lock()
	want := s.ControlToken
	s.mu.Unlock()
	if token == "" || subtle.ConstantTimeCompare([]byte(want), []byte(token)) != 1 {
		return ErrUnauthorized
	}
	m.destroyLocked(id)
	return nil
}

// PreparePrompt appends the user message, moves the session to ACTIVE and
// returns a copy of the full history for the provider.
func (m *Manager) PreparePrompt(id, content string) ([]provider.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != protocol.StateApproved {
		return nil, ErrNotApproved
	}
	s.History = append(s.History, provider.Message{Role: provider.RoleUser, Content: content})
	s.State = protocol.StateActive
	s.IdleDeadline = m.cfg.Clock.Now().Add(m.cfg.IdleTimeout)
	hist := make([]provider.Message, len(s.History))
	copy(hist, s.History)
	return hist, nil
}

// FinishPrompt stores the assistant reply, moves the session back to APPROVED
// and clears the in-flight generation.
func (m *Manager) FinishPrompt(id, assistantText, finishReason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if assistantText != "" {
		s.History = append(s.History, provider.Message{Role: provider.RoleAssistant, Content: assistantText})
	}
	s.State = protocol.StateApproved
	s.IdleDeadline = m.cfg.Clock.Now().Add(m.cfg.IdleTimeout)
	s.genCancel = nil
}

// FailPrompt removes the user message added by PreparePrompt and moves the
// session back to APPROVED so the user can retry.
func (m *Manager) FailPrompt(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if n := len(s.History); n > 0 && s.History[n-1].Role == provider.RoleUser {
		s.History = s.History[:n-1]
	}
	s.State = protocol.StateApproved
	s.IdleDeadline = m.cfg.Clock.Now().Add(m.cfg.IdleTimeout)
	s.genCancel = nil
}

// SetGenCancel registers the cancellation function of the in-flight
// generation. It is invoked by a later /cancel request.
func (m *Manager) SetGenCancel(id string, cancel func()) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.genCancel = cancel
	return nil
}

// CancelGeneration cancels the in-flight generation, if any.
func (m *Manager) CancelGeneration(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.genCancel == nil {
		return ErrNotActive
	}
	c := s.genCancel
	s.genCancel = nil
	c()
	return nil
}

// Destroy terminates the session, releases everything and removes it from
// all maps. No data survives.
func (m *Manager) Destroy(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.destroyLocked(id)
}

func (m *Manager) destroyLocked(id string) {
	s, ok := m.sessions[id]
	if !ok {
		return
	}
	s.mu.Lock()
	if s.genCancel != nil {
		s.genCancel()
	}
	s.State = protocol.StateDestroyed
	s.History = nil
	s.APIKey = ""
	s.ControlToken = ""
	s.PairToken = ""
	s.genCancel = nil
	code := s.PairCode
	s.PairCode = ""
	s.mu.Unlock()

	delete(m.sessions, id)
	delete(m.byPairCode, code)
}

// Sweep destroys sessions whose pairing expired or which were idle too long.
// ACTIVE sessions are left alone; the gateway enforces the generation timeout.
func (m *Manager) Sweep() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.cfg.Clock.Now()
	for id, s := range m.sessions {
		s.mu.Lock()
		state := s.State
		pairExp := now.After(s.PairExpiry)
		idleExp := now.After(s.IdleDeadline)
		s.mu.Unlock()

		expired := false
		switch state {
		case protocol.StateWaiting:
			expired = pairExp
		case protocol.StateApproved:
			expired = idleExp
		}
		if expired {
			m.destroyLocked(id)
		}
	}
}

// StartSweeper runs Sweep every second until ctx is cancelled.
func (m *Manager) StartSweeper(ctx context.Context) {
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				m.Sweep()
			}
		}
	}()
}

