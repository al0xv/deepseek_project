package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
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

// approveRaw performs the raw POST /v1/pair request (as the CLI/iPhone do).
func approveRaw(t *testing.T, baseURL, code string) *http.Response {
	t.Helper()
	body := bytes.NewReader([]byte(`{"pairing_code":"` + protocol.NormalizePairCode(code) + `"}`))
	resp, err := http.Post(baseURL+"/v1/pair", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestApproveReturnsControlToken(t *testing.T) {
	_, gc, _ := newTestGateway(t, session.Config{})
	sess, err := gc.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	resp := approveRaw(t, gc.BaseURL, sess.PairingCode)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var ar protocol.ApproveResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		t.Fatal(err)
	}
	if ar.SessionID != sess.SessionID {
		t.Fatalf("session_id = %q, want %q", ar.SessionID, sess.SessionID)
	}
	if ar.State != protocol.StateApproved {
		t.Fatalf("state = %q, want APPROVED", ar.State)
	}
	if ar.ControlToken == "" || len(ar.ControlToken) != 43 {
		t.Fatalf("control_token = %q, want 43 chars", ar.ControlToken)
	}
}

func endSessionRequest(t *testing.T, baseURL, id, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, baseURL+"/v1/sessions/"+id, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestEndSessionOverHTTP(t *testing.T) {
	_, gc, _ := newTestGateway(t, session.Config{})
	sess, err := gc.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	resp := approveRaw(t, gc.BaseURL, sess.PairingCode)
	var ar protocol.ApproveResponse
	_ = json.NewDecoder(resp.Body).Decode(&ar)
	resp.Body.Close()

	del := endSessionRequest(t, gc.BaseURL, sess.SessionID, ar.ControlToken)
	defer del.Body.Close()
	if del.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", del.StatusCode)
	}
	if _, err := gc.Status(sess.SessionID); err == nil {
		t.Fatal("session still exists after controller end")
	}
}

func TestEndSessionWrongToken(t *testing.T) {
	_, gc, _ := newTestGateway(t, session.Config{})
	sess, err := gc.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := gc.Approve(sess.PairingToken); err != nil {
		t.Fatal(err)
	}

	del := endSessionRequest(t, gc.BaseURL, sess.SessionID, "wrong-token")
	defer del.Body.Close()
	if del.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", del.StatusCode)
	}
	var eb protocol.ErrorBody
	_ = json.NewDecoder(del.Body).Decode(&eb)
	if eb.Code != protocol.ErrUnauthorized {
		t.Fatalf("error code = %q, want unauthorized", eb.Code)
	}
	// Session must still be alive.
	st, err := gc.Status(sess.SessionID)
	if err != nil || st != protocol.StateApproved {
		t.Fatalf("session after rejected end = %q, %v", st, err)
	}
}

func TestEndSessionMissingToken(t *testing.T) {
	_, gc, _ := newTestGateway(t, session.Config{})
	sess, err := gc.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := gc.Approve(sess.PairingToken); err != nil {
		t.Fatal(err)
	}

	del := endSessionRequest(t, gc.BaseURL, sess.SessionID, "")
	defer del.Body.Close()
	if del.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", del.StatusCode)
	}
}

func TestEndSessionUnknownSession(t *testing.T) {
	_, gc, _ := newTestGateway(t, session.Config{})
	del := endSessionRequest(t, gc.BaseURL, "nonexistent", "some-token")
	defer del.Body.Close()
	if del.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", del.StatusCode)
	}
}

func TestEndSessionOnceOnly(t *testing.T) {
	_, gc, _ := newTestGateway(t, session.Config{})
	sess, err := gc.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	resp := approveRaw(t, gc.BaseURL, sess.PairingCode)
	var ar protocol.ApproveResponse
	_ = json.NewDecoder(resp.Body).Decode(&ar)
	resp.Body.Close()

	if del := endSessionRequest(t, gc.BaseURL, sess.SessionID, ar.ControlToken); del.StatusCode != http.StatusNoContent {
		t.Fatalf("first end status = %d", del.StatusCode)
	} else {
		del.Body.Close()
	}
	// Repeating the end must not resurrect the session.
	del2 := endSessionRequest(t, gc.BaseURL, sess.SessionID, ar.ControlToken)
	defer del2.Body.Close()
	if del2.StatusCode != http.StatusNotFound {
		t.Fatalf("second end status = %d, want 404", del2.StatusCode)
	}
}

