package deepseek

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"deepseek-terminal/internal/provider"
)

// stubServer returns an httptest server that checks the request and writes
// the given SSE chunks.
func stubServer(t *testing.T, chunks []string, check func(*http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q, want /chat/completions", r.URL.Path)
		}
		if check != nil {
			check(r)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, c := range chunks {
			_, _ = w.Write([]byte(c))
		}
	}))
}

func TestStreamCompletion(t *testing.T) {
	srv := stubServer(t, []string{
		"data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n",
		"data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n",
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n",
		"data: [DONE]\n\n",
	}, func(r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("auth = %q", got)
		}
	})
	defer srv.Close()

	c := New("test-key")
	c.BaseURL = srv.URL

	var got []string
	res, err := c.StreamCompletion(context.Background(), provider.CompletionRequest{
		Model:    "deepseek-chat",
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	}, func(d string) error {
		got = append(got, d)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(got, ""); joined != "Hello" {
		t.Fatalf("deltas = %q", joined)
	}
	if res.FullText != "Hello" || res.FinishReason != provider.FinishStop {
		t.Fatalf("result = %+v", res)
	}
}

func TestStreamCompletionCancelReturnsPartial(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"par\"}}]}\n\n"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := New("k")
	c.BaseURL = srv.URL

	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan struct{})
	errCh := make(chan error, 1)
	resCh := make(chan provider.CompletionResult, 1)

	go func() {
		res, err := c.StreamCompletion(ctx, provider.CompletionRequest{}, func(string) error {
			select {
			case <-first:
			default:
				close(first)
			}
			return nil
		})
		errCh <- err
		resCh <- res
	}()

	<-first
	cancel()

	if err := <-errCh; err != nil {
		t.Fatalf("err = %v", err)
	}
	res := <-resCh
	if res.FinishReason != provider.FinishCanceled {
		t.Fatalf("finish = %q, want cancelled", res.FinishReason)
	}
	if res.FullText != "par" {
		t.Fatalf("partial text = %q, want par", res.FullText)
	}
}

func TestStreamCompletionHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadRequest)
	}))
	defer srv.Close()

	c := New("k")
	c.BaseURL = srv.URL
	_, err := c.StreamCompletion(context.Background(), provider.CompletionRequest{}, nil)
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("err = %v", err)
	}
}
