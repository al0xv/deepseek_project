package deepseek

import (
	"context"
	"encoding/json"
	"io"
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
			t.Errorf("auth = %q, want only the Authorization header carrying the key", got)
		}
		// The API key must never appear anywhere else in the outbound request.
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		if strings.Contains(string(body), "test-key") {
			t.Errorf("API key leaked into request body: %s", body)
		}
	})
	defer srv.Close()

	c := New("test-key")
	c.BaseURL = srv.URL

	var got []string
	res, err := c.StreamCompletion(context.Background(), provider.CompletionRequest{
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

	key := "sk-super-secret-test-key"
	c := New(key)
	c.BaseURL = srv.URL
	_, err := c.StreamCompletion(context.Background(), provider.CompletionRequest{}, nil)
	if err == nil {
		t.Fatal("expected an error for non-2xx upstream response")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Fatalf("err = %v, want upstream HTTP status surfaced", err)
	}
	if strings.Contains(err.Error(), key) {
		t.Fatalf("upstream error leaks the API key: %v", err)
	}
}

// TestStreamCompletionRequestBody pins the exact semantic DeepSeek request
// body for each supported settings combination.
func TestStreamCompletionRequestBody(t *testing.T) {
	cases := []struct {
		name       string
		settings   provider.GenerationSettings
		wantModel  string
		wantType   string
		wantEffort string
	}{
		{"FlashHigh", provider.GenerationSettings{Model: provider.ModelV4Flash, ThinkingEnabled: true, ReasoningEffort: provider.ReasoningHigh}, "deepseek-v4-flash", "enabled", "high"},
		{"ProMax", provider.GenerationSettings{Model: provider.ModelV4Pro, ThinkingEnabled: true, ReasoningEffort: provider.ReasoningMax}, "deepseek-v4-pro", "enabled", "max"},
		{"FlashLow", provider.GenerationSettings{Model: provider.ModelV4Flash, ThinkingEnabled: true, ReasoningEffort: provider.ReasoningLow}, "deepseek-v4-flash", "enabled", "low"},
		{"FlashOff", provider.GenerationSettings{Model: provider.ModelV4Flash, ThinkingEnabled: false, ReasoningEffort: ""}, "deepseek-v4-flash", "disabled", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotBody, _ = io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = w.Write([]byte("data: [DONE]\n\n"))
			}))
			defer srv.Close()

			c := New("test-key")
			c.BaseURL = srv.URL
			_, err := c.StreamCompletion(context.Background(), provider.CompletionRequest{
				Settings: tc.settings,
				Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
			}, nil)
			if err != nil {
				t.Fatal(err)
			}

			var body struct {
				Model    string `json:"model"`
				Stream   bool   `json:"stream"`
				Thinking *struct {
					Type string `json:"type"`
				} `json:"thinking"`
				ReasoningEffort string `json:"reasoning_effort"`
			}
			if err := json.Unmarshal(gotBody, &body); err != nil {
				t.Fatal(err)
			}
			if body.Model != tc.wantModel {
				t.Errorf("model = %q, want %q", body.Model, tc.wantModel)
			}
			if !body.Stream {
				t.Error("stream must be true")
			}
			if body.Thinking == nil || body.Thinking.Type != tc.wantType {
				t.Errorf("thinking = %+v, want type %q", body.Thinking, tc.wantType)
			}
			if body.ReasoningEffort != tc.wantEffort {
				t.Errorf("reasoning_effort = %q, want %q", body.ReasoningEffort, tc.wantEffort)
			}
		})
	}
}

func TestStreamCompletionFallsBackToDefaultsForZeroSettings(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	c := New("test-key")
	c.BaseURL = srv.URL
	if _, err := c.StreamCompletion(context.Background(), provider.CompletionRequest{}, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gotBody), `"model":"deepseek-v4-flash"`) {
		t.Fatalf("zero settings must fall back to deepseek-v4-flash: %s", gotBody)
	}
	if !strings.Contains(string(gotBody), `"type":"enabled"`) {
		t.Fatalf("zero settings must default to thinking enabled: %s", gotBody)
	}
	if !strings.Contains(string(gotBody), `"reasoning_effort":"high"`) {
		t.Fatalf("zero settings must default to high effort: %s", gotBody)
	}
}
