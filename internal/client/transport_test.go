package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"deepseek-terminal/internal/protocol"
)

func fakeServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/sessions":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(protocol.SessionCreateResponse{
				SessionID:    "sess123",
				PairingToken: "tok",
				PairingCode:  "472 913",
				GatewayURL:   "http://fake",
				ExpiresIn:    120,
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/sessions/sess123":
			_ = json.NewEncoder(w).Encode(protocol.SessionStatusResponse{SessionID: "sess123", State: protocol.StateApproved})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/sessions/sess123/prompt":
			w.Header().Set("Content-Type", "text/event-stream")
			_ = protocol.WriteSSE(w, "token", protocol.TokenEvent{Delta: "hel"})
			_ = protocol.WriteSSE(w, "token", protocol.TokenEvent{Delta: "lo"})
			_ = protocol.WriteSSE(w, "done", protocol.DoneEvent{FinishReason: "stop"})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/sessions/sess123/cancel":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/sessions/sess123/close":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
}

func TestCreateSession(t *testing.T) {
	srv := fakeServer(t)
	defer srv.Close()

	gc := NewGatewayClient(srv.URL)
	sess, err := gc.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	if sess.SessionID != "sess123" || sess.PairingCode != "472 913" {
		t.Fatalf("sess = %+v", sess)
	}
}

func TestStatus(t *testing.T) {
	srv := fakeServer(t)
	defer srv.Close()

	gc := NewGatewayClient(srv.URL)
	st, err := gc.Status("sess123")
	if err != nil {
		t.Fatal(err)
	}
	if st != protocol.StateApproved {
		t.Fatalf("state = %q", st)
	}
}

func TestPromptStreams(t *testing.T) {
	srv := fakeServer(t)
	defer srv.Close()

	gc := NewGatewayClient(srv.URL)
	var got strings.Builder
	finish, err := gc.Prompt(context.Background(), "sess123", "hello", func(d string) error {
		got.WriteString(d)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if finish != "stop" {
		t.Fatalf("finish = %q", finish)
	}
	if got.String() != "hello" {
		t.Fatalf("deltas = %q", got.String())
	}
}

func TestCancelAndClose(t *testing.T) {
	srv := fakeServer(t)
	defer srv.Close()

	gc := NewGatewayClient(srv.URL)
	if err := gc.Cancel("sess123"); err != nil {
		t.Fatal(err)
	}
	if err := gc.Close("sess123"); err != nil {
		t.Fatal(err)
	}
}

func TestErrorBodyMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTestError(w, http.StatusForbidden, protocol.ErrPairingPending, "waiting")
	}))
	defer srv.Close()

	gc := NewGatewayClient(srv.URL)
	_, err := gc.Prompt(context.Background(), "x", "hi", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "pairing_pending") {
		t.Fatalf("err = %v", err)
	}
}

func writeTestError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(protocol.ErrorBody{Code: code, Message: msg})
}
