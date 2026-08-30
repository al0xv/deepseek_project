package client

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"deepseek-terminal/internal/protocol"
)

// fakeClient is a scripted Client for REPL tests.
type fakeClient struct {
	createResp protocol.SessionCreateResponse
	createErr  error

	mu       sync.Mutex
	states   []protocol.SessionState
	prompts  []string
	promptFn func(id, content string) (string, error)

	cancelCalled atomic.Bool
	closeCalled  atomic.Bool
}

func (f *fakeClient) CreateSession() (protocol.SessionCreateResponse, error) {
	return f.createResp, f.createErr
}

func (f *fakeClient) Status(id string) (protocol.SessionState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.states) == 0 {
		return protocol.StateApproved, nil
	}
	st := f.states[0]
	f.states = f.states[1:]
	return st, nil
}

func (f *fakeClient) Prompt(ctx context.Context, id, content string, onDelta func(string) error) (string, error) {
	f.mu.Lock()
	f.prompts = append(f.prompts, content)
	fn := f.promptFn
	f.mu.Unlock()
	if fn != nil {
		return fn(id, content)
	}
	for _, ch := range "mock" {
		if err := onDelta(string(ch)); err != nil {
			return "", err
		}
	}
	return "stop", nil
}

func (f *fakeClient) Cancel(id string) error {
	f.cancelCalled.Store(true)
	return nil
}

func (f *fakeClient) Close(id string) error {
	f.closeCalled.Store(true)
	return nil
}

func newFakeREPL(fc *fakeClient, input string) (*REPL, *bytes.Buffer) {
	out := &bytes.Buffer{}
	r := NewREPL(fc, strings.NewReader(input), out)
	r.enableQR = false
	return r, out
}

// noSignals returns a signal channel that is never written to.
func noSignals() chan os.Signal {
	return make(chan os.Signal, 1)
}

func TestREPLFullLifecycle(t *testing.T) {
	fc := &fakeClient{
		createResp: protocol.SessionCreateResponse{
			SessionID: "s1", PairingToken: "t", PairingCode: "472 913", ExpiresIn: 120,
		},
		states: []protocol.SessionState{protocol.StateWaiting, protocol.StateApproved},
	}
	r, out := newFakeREPL(fc, "hello\n/exit\n")
	if err := r.Run(context.Background(), noSignals()); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	for _, want := range []string{"WAITING", "Code: 472 913", "Approved", "DeepSeek ready.", "You > ", "DeepSeek > ", "mock", "Session destroyed."} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if len(fc.prompts) != 1 || fc.prompts[0] != "hello" {
		t.Fatalf("prompts = %v", fc.prompts)
	}
	if !fc.closeCalled.Load() {
		t.Fatal("session was not closed")
	}
}

func TestREPLEmptyLineIgnored(t *testing.T) {
	fc := &fakeClient{createResp: protocol.SessionCreateResponse{SessionID: "s1", PairingCode: "472 913", ExpiresIn: 120}}
	r, _ := newFakeREPL(fc, "\n/exit\n")
	if err := r.Run(context.Background(), noSignals()); err != nil {
		t.Fatal(err)
	}
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if len(fc.prompts) != 0 {
		t.Fatalf("prompts = %v, want none", fc.prompts)
	}
}

func TestREPLCancelledPrompt(t *testing.T) {
	fc := &fakeClient{
		createResp: protocol.SessionCreateResponse{SessionID: "s1", PairingCode: "472 913", ExpiresIn: 120},
	}
	fc.promptFn = func(id, content string) (string, error) {
		return "cancelled", nil
	}
	r, out := newFakeREPL(fc, "hello\n/exit\n")
	if err := r.Run(context.Background(), noSignals()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "[cancelled]") {
		t.Fatalf("missing [cancelled]:\n%s", out.String())
	}
}

func TestREPLPromptErrorKeepsRunning(t *testing.T) {
	fc := &fakeClient{
		createResp: protocol.SessionCreateResponse{SessionID: "s1", PairingCode: "472 913", ExpiresIn: 120},
	}
	fc.promptFn = func(id, content string) (string, error) {
		if content == "boom" {
			return "", errors.New("upstream_error: nope")
		}
		return "stop", nil
	}
	r, out := newFakeREPL(fc, "boom\nok\n/exit\n")
	if err := r.Run(context.Background(), noSignals()); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "Error: upstream_error: nope") {
		t.Fatalf("missing error output:\n%s", got)
	}
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if len(fc.prompts) != 2 {
		t.Fatalf("prompts = %v, want 2 (error did not kill the loop)", fc.prompts)
	}
}

func TestREPLInterruptDuringStreamingCancels(t *testing.T) {
	fc := &fakeClient{
		createResp: protocol.SessionCreateResponse{SessionID: "s1", PairingCode: "472 913", ExpiresIn: 120},
	}
	var entered, release = make(chan struct{}), make(chan struct{})
	fc.promptFn = func(id, content string) (string, error) {
		close(entered)
		<-release
		return "cancelled", nil
	}

	r, out := newFakeREPL(fc, "hello\n/exit\n")
	sig := make(chan os.Signal, 4)
	done := make(chan error, 1)
	go func() {
		done <- r.Run(context.Background(), sig)
	}()

	<-entered
	// First Ctrl+C while streaming: cancels the generation.
	sig <- os.Interrupt
	deadline := time.Now().Add(2 * time.Second)
	for !fc.cancelCalled.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !fc.cancelCalled.Load() {
		t.Fatal("Cancel() was not called on interrupt")
	}
	close(release) // now let the (already cancelled) prompt finish
	<-done

	if !strings.Contains(out.String(), "[cancelled]") {
		t.Fatalf("missing [cancelled]:\n%s", out.String())
	}
}
