package gateway

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"deepseek-terminal/internal/client"
	"deepseek-terminal/internal/protocol"
	"deepseek-terminal/internal/provider"
	"deepseek-terminal/internal/provider/mock"
	"deepseek-terminal/internal/session"
)

func newGatewayServer(t *testing.T, p provider.Provider, mgrCfg session.Config) (*session.Manager, *client.GatewayClient) {
	t.Helper()
	mgr := session.New(mgrCfg)
	g := New(Config{
		Provider:   p,
		Model:      "deepseek-chat",
		Manager:    mgr,
		GatewayURL: "http://gw.local",
	})
	srv := httptest.NewServer(g.Handler())
	t.Cleanup(srv.Close)
	return mgr, client.NewGatewayClient(srv.URL)
}

func newTestGateway(t *testing.T, mgrCfg session.Config) (*session.Manager, *client.GatewayClient, string) {
	t.Helper()
	mgr, gc := newGatewayServer(t, &mock.Provider{}, mgrCfg)
	return mgr, gc, gc.BaseURL
}

// newTestGatewayWithDelay uses a slow mock so cancel/disconnect tests can
// interrupt a generation that is still in flight.
func newTestGatewayWithDelay(t *testing.T) (*session.Manager, *client.GatewayClient, string) {
	t.Helper()
	mgr, gc := newGatewayServer(t, &mock.Provider{Delay: 5 * time.Millisecond}, session.Config{})
	return mgr, gc, gc.BaseURL
}

func TestSessionLifecycleOverHTTP(t *testing.T) {
	mgr, gc, _ := newTestGateway(t, session.Config{})

	// Create.
	sess, err := gc.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	if sess.PairingToken == "" || sess.PairingCode == "" {
		t.Fatal("missing pairing credentials")
	}
	if sess.ExpiresIn != 120 {
		t.Fatalf("expires_in = %d, want 120", sess.ExpiresIn)
	}

	// Status is WAITING.
	st, err := gc.Status(sess.SessionID)
	if err != nil || st != protocol.StateWaiting {
		t.Fatalf("status = %q, %v", st, err)
	}

	// Prompt before approve is forbidden.
	if _, err := gc.Prompt(context.Background(), sess.SessionID, "hi", nil); err == nil {
		t.Fatal("expected error for unapproved session")
	} else if !strings.Contains(err.Error(), "pairing_pending") {
		t.Fatalf("err = %v, want pairing_pending", err)
	}

	// Approve by code.
	if err := gc.Approve(sess.PairingToken); err != nil {
		t.Fatal(err)
	}
	st, _ = gc.Status(sess.SessionID)
	if st != protocol.StateApproved {
		t.Fatalf("status = %q, want APPROVED", st)
	}

	// Prompt streams a reply.
	var got strings.Builder
	finish, err := gc.Prompt(context.Background(), sess.SessionID, "hello", func(d string) error {
		got.WriteString(d)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if finish != "stop" {
		t.Fatalf("finish = %q, want stop", finish)
	}
	if !strings.Contains(got.String(), "mock reply to: hello") {
		t.Fatalf("reply = %q", got.String())
	}

	// Session is back to APPROVED and history grew.
	st, _ = gc.Status(sess.SessionID)
	if st != protocol.StateApproved {
		t.Fatalf("status = %q, want APPROVED", st)
	}
	if n := len(mgr.SessionHistory(sess.SessionID)); n != 2 {
		t.Fatalf("history = %d messages, want 2", n)
	}

	// Second prompt keeps history.
	if _, err := gc.Prompt(context.Background(), sess.SessionID, "again", nil); err != nil {
		t.Fatal(err)
	}
	if n := len(mgr.SessionHistory(sess.SessionID)); n != 4 {
		t.Fatalf("history = %d messages, want 4", n)
	}

	// Close destroys the session.
	if err := gc.Close(sess.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := gc.Status(sess.SessionID); err == nil {
		t.Fatal("session still exists after close")
	}
}

func TestCancelGenerationOverHTTP(t *testing.T) {
	_, gc, _ := newTestGatewayWithDelay(t)
	sess, err := gc.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := gc.Approve(sess.PairingToken); err != nil {
		t.Fatal(err)
	}

	first := make(chan struct{})
	done := make(chan struct{})
	go func() {
		finish, err := gc.Prompt(context.Background(), sess.SessionID, "a fairly long prompt", func(d string) error {
			select {
			case <-first:
			default:
				close(first)
			}
			return nil
		})
		if err != nil {
			t.Errorf("prompt err = %v", err)
		}
		if finish != "cancelled" {
			t.Errorf("finish = %q, want cancelled", finish)
		}
		close(done)
	}()

	select {
	case <-first:
	case <-time.After(3 * time.Second):
		t.Fatal("no first token")
	}

	if err := gc.Cancel(sess.SessionID); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("prompt did not finish after cancel")
	}
}

func TestDisconnectDestroysSession(t *testing.T) {
	_, gc, _ := newTestGatewayWithDelay(t)
	sess, err := gc.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := gc.Approve(sess.PairingToken); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_, _ = gc.Prompt(ctx, sess.SessionID, "long text", nil)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond) // let the generation start
	cancel()                            // simulate client disconnect
	<-done

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := gc.Status(sess.SessionID); err != nil {
			return // session destroyed
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("session not destroyed after disconnect")
}
