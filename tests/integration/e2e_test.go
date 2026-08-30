// Package integration contains end-to-end tests that run a real gateway
// with a mock provider and exercise the full pairing lifecycle over HTTP/SSE.
package integration

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"deepseek-terminal/internal/client"
	"deepseek-terminal/internal/gateway"
	"deepseek-terminal/internal/protocol"
	"deepseek-terminal/internal/provider/mock"
	"deepseek-terminal/internal/session"
)

func newServer(t *testing.T, mp *mock.Provider) (*session.Manager, *client.GatewayClient, *httptest.Server) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	mgr := session.New(session.Config{})
	mgr.StartSweeper(ctx)
	g := gateway.New(gateway.Config{
		Provider:   mp,
		Model:      "deepseek-chat",
		Manager:    mgr,
		GatewayURL: "http://gw.local",
	})
	srv := httptest.NewServer(g.Handler())
	t.Cleanup(srv.Close)
	return mgr, client.NewGatewayClient(srv.URL), srv
}

func TestFullPairingLifecycle(t *testing.T) {
	mp := &mock.Provider{}
	_, gc, _ := newServer(t, mp)

	// 1. Create.
	sess, err := gc.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	if sess.PairingToken == "" || sess.PairingCode == "" {
		t.Fatal("missing pairing credentials")
	}

	// 2. WAITING.
	st, err := gc.Status(sess.SessionID)
	if err != nil || st != protocol.StateWaiting {
		t.Fatalf("status = %q, %v", st, err)
	}

	// 3. Prompt before approval is rejected.
	if _, err := gc.Prompt(context.Background(), sess.SessionID, "hi", nil); err == nil {
		t.Fatal("expected rejection before approval")
	}

	// 4. Approve with the token from the QR payload.
	if err := gc.Approve(sess.PairingToken); err != nil {
		t.Fatal(err)
	}
	st, _ = gc.Status(sess.SessionID)
	if st != protocol.StateApproved {
		t.Fatalf("status = %q, want APPROVED", st)
	}

	// 5. First prompt streams a full reply.
	var reply strings.Builder
	finish, err := gc.Prompt(context.Background(), sess.SessionID, "explain decorators", func(d string) error {
		reply.WriteString(d)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if finish != "stop" {
		t.Fatalf("finish = %q", finish)
	}
	if !strings.Contains(reply.String(), "mock reply to: explain decorators") {
		t.Fatalf("reply = %q", reply.String())
	}

	// 6. Second prompt sees the grown history (user, assistant, user).
	if _, err := gc.Prompt(context.Background(), sess.SessionID, "example please", nil); err != nil {
		t.Fatal(err)
	}
	if n := len(mp.LastRequest().Messages); n != 3 {
		t.Fatalf("history sent to provider = %d messages, want 3", n)
	}

	// 7. Close destroys the session.
	if err := gc.Close(sess.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := gc.Status(sess.SessionID); err == nil {
		t.Fatal("session still exists after close")
	}
}

func TestCancelDuringStreaming(t *testing.T) {
	mp := &mock.Provider{Delay: 5 * time.Millisecond}
	_, gc, _ := newServer(t, mp)

	sess, err := gc.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := gc.Approve(sess.PairingToken); err != nil {
		t.Fatal(err)
	}

	first := make(chan struct{})
	done := make(chan error, 1)
	finishCh := make(chan string, 1)
	go func() {
		finish, err := gc.Prompt(context.Background(), sess.SessionID, "please keep generating", func(string) error {
			select {
			case <-first:
			default:
				close(first)
			}
			return nil
		})
		finishCh <- finish
		done <- err
	}()

	select {
	case <-first:
	case <-time.After(3 * time.Second):
		t.Fatal("no first delta")
	}

	if err := gc.Cancel(sess.SessionID); err != nil {
		t.Fatal(err)
	}

	select {
	case finish := <-finishCh:
		if finish != "cancelled" {
			t.Fatalf("finish = %q, want cancelled", finish)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("generation did not finish after cancel")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	// Session survives cancellation: it can accept another prompt.
	if _, err := gc.Prompt(context.Background(), sess.SessionID, "again", nil); err != nil {
		t.Fatalf("session unusable after cancel: %v", err)
	}
}

func TestIdleTimeoutDestroysSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mgr := session.New(session.Config{IdleTimeout: 50 * time.Millisecond})
	mgr.StartSweeper(ctx)
	mp := &mock.Provider{}
	g := gateway.New(gateway.Config{Provider: mp, Manager: mgr, GatewayURL: "http://gw.local"})
	srv := httptest.NewServer(g.Handler())
	t.Cleanup(srv.Close)
	gc := client.NewGatewayClient(srv.URL)

	sess, err := gc.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := gc.Approve(sess.PairingToken); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := gc.Status(sess.SessionID); err != nil {
			return // destroyed
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("idle session was not destroyed")
}

func TestPairingExpiryDestroysSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mgr := session.New(session.Config{PairTimeout: 50 * time.Millisecond})
	mgr.StartSweeper(ctx)
	mp := &mock.Provider{}
	g := gateway.New(gateway.Config{Provider: mp, Manager: mgr, GatewayURL: "http://gw.local"})
	srv := httptest.NewServer(g.Handler())
	t.Cleanup(srv.Close)
	gc := client.NewGatewayClient(srv.URL)

	sess, err := gc.CreateSession()
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := gc.Status(sess.SessionID); err != nil {
			return // destroyed by pair expiry
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("unapproved session was not destroyed")
}
