package session

import (
	"testing"
	"time"

	"deepseek-terminal/internal/provider"
)

type fakeClock struct{ t time.Time }

func (f *fakeClock) Now() time.Time { return f.t }

func (f *fakeClock) Advance(d time.Duration) { f.t = f.t.Add(d) }

func newTestManager(pair, idle time.Duration, max int) (*Manager, *fakeClock) {
	fc := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	m := New(Config{
		Clock:       fc,
		PairTimeout: pair,
		IdleTimeout: idle,
		GenTimeout:  time.Minute,
		MaxSessions: max,
	})
	return m, fc
}

func TestCreateSessionIsWaiting(t *testing.T) {
	m, _ := newTestManager(time.Minute, time.Minute, 4)
	s, err := m.Create()
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateWaiting {
		t.Fatalf("state = %q, want WAITING", s.State)
	}
	if s.PairToken == "" || s.PairCode == "" {
		t.Fatal("pairing credentials missing")
	}
	if len(s.PairToken) != 43 || len(s.PairCode) != 6 {
		t.Fatalf("pairing credentials malformed: token=%d code=%d", len(s.PairToken), len(s.PairCode))
	}
}

func TestApproveByTokenAndCode(t *testing.T) {
	m, _ := newTestManager(time.Minute, time.Minute, 4)
	s, _ := m.Create()

	got, err := m.ApproveByToken(s.PairToken)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateApproved {
		t.Fatalf("state = %q, want APPROVED", got.State)
	}

	s2, _ := m.Create()
	if _, err := m.ApproveByCode(s2.PairCode); err != nil {
		t.Fatal(err)
	}
}

func TestApproveWrongCredentials(t *testing.T) {
	m, _ := newTestManager(time.Minute, time.Minute, 4)
	if _, err := m.ApproveByToken("nope"); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if _, err := m.ApproveByCode("000000"); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestApproveTwiceFails(t *testing.T) {
	m, _ := newTestManager(time.Minute, time.Minute, 4)
	s, _ := m.Create()
	if _, err := m.ApproveByToken(s.PairToken); err != nil {
		t.Fatal(err)
	}
	if _, err := m.ApproveByToken(s.PairToken); err != ErrAlreadyApproved {
		t.Fatalf("err = %v, want ErrAlreadyApproved", err)
	}
}

func TestApproveExpiredPairing(t *testing.T) {
	m, fc := newTestManager(100*time.Second, time.Minute, 4)
	s, _ := m.Create()
	fc.Advance(101 * time.Second)
	if _, err := m.ApproveByToken(s.PairToken); err != ErrPairingExpired {
		t.Fatalf("err = %v, want ErrPairingExpired", err)
	}
}

func TestSweepDestroysExpiredPairing(t *testing.T) {
	m, fc := newTestManager(100*time.Second, time.Minute, 4)
	s, _ := m.Create()
	fc.Advance(101 * time.Second)
	m.Sweep()
	if _, err := m.Status(s.ID); err != ErrNotFound {
		t.Fatalf("session survived pair expiry: %v", err)
	}
}

func TestSweepDestroysIdleApproved(t *testing.T) {
	m, fc := newTestManager(time.Minute, 10*time.Second, 4)
	s, _ := m.Create()
	if _, err := m.ApproveByCode(s.PairCode); err != nil {
		t.Fatal(err)
	}
	fc.Advance(11 * time.Second)
	m.Sweep()
	if _, err := m.Status(s.ID); err != ErrNotFound {
		t.Fatalf("session survived idle timeout: %v", err)
	}
}

func TestPromptLifecycleKeepsHistory(t *testing.T) {
	m, _ := newTestManager(time.Minute, time.Minute, 4)
	s, _ := m.Create()
	if _, err := m.ApproveByToken(s.PairToken); err != nil {
		t.Fatal(err)
	}

	hist, err := m.PreparePrompt(s.ID, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 1 || hist[0].Role != provider.RoleUser || hist[0].Content != "hello" {
		t.Fatalf("hist = %+v", hist)
	}
	if st, _ := m.Status(s.ID); st != StateActive {
		t.Fatalf("state = %q, want ACTIVE", st)
	}

	m.FinishPrompt(s.ID, "world", provider.FinishStop)

	if st, _ := m.Status(s.ID); st != StateApproved {
		t.Fatalf("state = %q, want APPROVED", st)
	}
	hist2, err := m.PreparePrompt(s.ID, "again")
	if err != nil {
		t.Fatal(err)
	}
	if len(hist2) != 3 {
		t.Fatalf("history length = %d, want 3 (user, assistant, user)", len(hist2))
	}
}

func TestPromptBeforeApproveFails(t *testing.T) {
	m, _ := newTestManager(time.Minute, time.Minute, 4)
	s, _ := m.Create()
	if _, err := m.PreparePrompt(s.ID, "hi"); err != ErrNotApproved {
		t.Fatalf("err = %v, want ErrNotApproved", err)
	}
}

func TestFailPromptPopsUserMessage(t *testing.T) {
	m, _ := newTestManager(time.Minute, time.Minute, 4)
	s, _ := m.Create()
	_, _ = m.ApproveByCode(s.PairCode)
	_, _ = m.PreparePrompt(s.ID, "boom")
	m.FailPrompt(s.ID)

	hist, err := m.PreparePrompt(s.ID, "retry")
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 1 || hist[0].Content != "retry" {
		t.Fatalf("history after fail+retry = %+v", hist)
	}
}

func TestCancelGeneration(t *testing.T) {
	m, _ := newTestManager(time.Minute, time.Minute, 4)
	s, _ := m.Create()
	_, _ = m.ApproveByCode(s.PairCode)

	cancelled := false
	if err := m.SetGenCancel(s.ID, func() { cancelled = true }); err != nil {
		t.Fatal(err)
	}
	if err := m.CancelGeneration(s.ID); err != nil {
		t.Fatal(err)
	}
	if !cancelled {
		t.Fatal("genCancel was not invoked")
	}
	if err := m.CancelGeneration(s.ID); err != ErrNotActive {
		t.Fatalf("second cancel err = %v, want ErrNotActive", err)
	}
}

func TestDestroyCleansEverything(t *testing.T) {
	m, _ := newTestManager(time.Minute, time.Minute, 4)
	s, _ := m.Create()
	_, _ = m.ApproveByCode(s.PairCode)
	_, _ = m.PreparePrompt(s.ID, "hi")
	m.FinishPrompt(s.ID, "yo", provider.FinishStop)

	m.Destroy(s.ID)

	if _, err := m.Status(s.ID); err != ErrNotFound {
		t.Fatalf("session still present: %v", err)
	}
	if s.History != nil {
		t.Fatal("history was not released")
	}
	if s.APIKey != "" || s.PairToken != "" || s.PairCode != "" {
		t.Fatal("secrets were not cleared")
	}
	// The code must no longer approve anything.
	if _, err := m.ApproveByCode(s.PairCode); err != ErrNotFound {
		t.Fatalf("destroyed pairing code still works: %v", err)
	}
}

func TestMaxSessions(t *testing.T) {
	m, _ := newTestManager(time.Minute, time.Minute, 2)
	if _, err := m.Create(); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Create(); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Create(); err != ErrMaxSessions {
		t.Fatalf("err = %v, want ErrMaxSessions", err)
	}
}

func TestDestroyDuringActiveGenerationCancels(t *testing.T) {
	m, _ := newTestManager(time.Minute, time.Minute, 4)
	s, _ := m.Create()
	_, _ = m.ApproveByCode(s.PairCode)

	cancelled := false
	_ = m.SetGenCancel(s.ID, func() { cancelled = true })
	m.Destroy(s.ID)
	if !cancelled {
		t.Fatal("destroy did not cancel in-flight generation")
	}
}

func TestApproveGeneratesControlToken(t *testing.T) {
	m, _ := newTestManager(time.Minute, time.Minute, 4)
	s, _ := m.Create()
	if _, err := m.ApproveByCode(s.PairCode); err != nil {
		t.Fatal(err)
	}
	tok, err := m.ControlToken(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if tok == "" {
		t.Fatal("control token is empty")
	}
	if len(tok) != 43 {
		t.Fatalf("control token length = %d, want 43 (32 bytes base64url)", len(tok))
	}
}

func TestControlTokensAreUnique(t *testing.T) {
	m, _ := newTestManager(time.Minute, time.Minute, 20)
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		s, _ := m.Create()
		if _, err := m.ApproveByCode(s.PairCode); err != nil {
			t.Fatal(err)
		}
		tok, _ := m.ControlToken(s.ID)
		if seen[tok] {
			t.Fatal("duplicate control token")
		}
		seen[tok] = true
	}
}

func TestEndByControlTokenValid(t *testing.T) {
	m, _ := newTestManager(time.Minute, time.Minute, 4)
	s, _ := m.Create()
	_, _ = m.ApproveByCode(s.PairCode)
	tok, _ := m.ControlToken(s.ID)

	if err := m.EndByControlToken(s.ID, tok); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Status(s.ID); err != ErrNotFound {
		t.Fatalf("session not destroyed: %v", err)
	}
}

func TestEndByControlTokenWrongToken(t *testing.T) {
	m, _ := newTestManager(time.Minute, time.Minute, 4)
	s, _ := m.Create()
	_, _ = m.ApproveByCode(s.PairCode)

	if err := m.EndByControlToken(s.ID, "wrong-token"); err != ErrUnauthorized {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
	if st, _ := m.Status(s.ID); st != StateApproved {
		t.Fatalf("session state = %q after rejected end, want APPROVED", st)
	}
}

func TestEndByControlTokenEmptyToken(t *testing.T) {
	m, _ := newTestManager(time.Minute, time.Minute, 4)
	s, _ := m.Create()
	_, _ = m.ApproveByCode(s.PairCode)
	if err := m.EndByControlToken(s.ID, ""); err != ErrUnauthorized {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestEndByControlTokenCrossSession(t *testing.T) {
	m, _ := newTestManager(time.Minute, time.Minute, 4)
	a, _ := m.Create()
	b, _ := m.Create()
	_, _ = m.ApproveByCode(a.PairCode)
	_, _ = m.ApproveByCode(b.PairCode)
	tokA, _ := m.ControlToken(a.ID)

	// Token from session A must not control session B.
	if err := m.EndByControlToken(b.ID, tokA); err != ErrUnauthorized {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
	if st, _ := m.Status(b.ID); st != StateApproved {
		t.Fatalf("session B state = %q, want APPROVED", st)
	}
	// And A itself still works.
	if err := m.EndByControlToken(a.ID, tokA); err != nil {
		t.Fatalf("session A end failed: %v", err)
	}
}

func TestEndByControlTokenDestroyedSession(t *testing.T) {
	m, _ := newTestManager(time.Minute, time.Minute, 4)
	s, _ := m.Create()
	_, _ = m.ApproveByCode(s.PairCode)
	tok, _ := m.ControlToken(s.ID)

	m.Destroy(s.ID)
	if err := m.EndByControlToken(s.ID, tok); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestControlTokenClearedAfterDestroy(t *testing.T) {
	m, _ := newTestManager(time.Minute, time.Minute, 4)
	s, _ := m.Create()
	_, _ = m.ApproveByCode(s.PairCode)
	if _, _ = m.ControlToken(s.ID); s.ControlToken == "" {
		t.Fatal("control token not set after approve")
	}
	m.Destroy(s.ID)
	if s.ControlToken != "" {
		t.Fatal("control token not cleared after destroy")
	}
}

