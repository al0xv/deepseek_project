// Package deepseek implements provider.Provider for the DeepSeek API
// (OpenAI-compatible chat completions with streaming).
package deepseek

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"deepseek-terminal/internal/provider"
)

const (
	defaultBaseURL = "https://api.deepseek.com"
	defaultModel   = "deepseek-chat"
)

// Client streams chat completions from the DeepSeek API.
type Client struct {
	APIKey  string
	BaseURL string
	Model   string
	HTTP    *http.Client
}

// New returns a DeepSeek client using the standard API base URL.
func New(apiKey string) *Client {
	return &Client{
		APIKey:  apiKey,
		BaseURL: defaultBaseURL,
		Model:   defaultModel,
		HTTP:    &http.Client{},
	}
}

type chatCompletionRequest struct {
	Model    string             `json:"model"`
	Messages []provider.Message `json:"messages"`
	Stream   bool               `json:"stream"`
}

// streamChunk mirrors one SSE data chunk of the OpenAI-compatible API.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

// StreamCompletion implements provider.Provider.
func (c *Client) StreamCompletion(ctx context.Context, req provider.CompletionRequest, onDelta func(delta string) error) (provider.CompletionResult, error) {
	model := c.Model
	if model == "" {
		model = defaultModel
	}
	payload, err := json.Marshal(chatCompletionRequest{Model: model, Messages: req.Messages, Stream: true})
	if err != nil {
		return provider.CompletionResult{}, fmt.Errorf("deepseek: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(c.BaseURL, "/")+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return provider.CompletionResult{}, fmt.Errorf("deepseek: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return provider.CompletionResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return provider.CompletionResult{}, fmt.Errorf("deepseek: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var sb strings.Builder
	finish := provider.FinishStop
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for sc.Scan() {
		if err := ctx.Err(); err != nil {
			return cancelled(sb.String(), finish), nil
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var ch streamChunk
		if err := json.Unmarshal([]byte(data), &ch); err != nil {
			return provider.CompletionResult{}, fmt.Errorf("deepseek: decode chunk: %w", err)
		}
		for _, c := range ch.Choices {
			if c.FinishReason != nil && *c.FinishReason != "" {
				finish = *c.FinishReason
			}
			if c.Delta.Content == "" {
				continue
			}
			if onDelta != nil {
				if err := onDelta(c.Delta.Content); err != nil {
					return provider.CompletionResult{}, err
				}
			}
			sb.WriteString(c.Delta.Content)
		}
	}

	if err := sc.Err(); err != nil {
		// The request may have been cancelled mid-read; treat it as a
		// graceful cancellation rather than a transport failure.
		if ctx.Err() != nil {
			return cancelled(sb.String(), finish), nil
		}
		return provider.CompletionResult{}, fmt.Errorf("deepseek: read stream: %w", err)
	}
	return provider.CompletionResult{FullText: sb.String(), FinishReason: finish}, nil
}

func cancelled(fullText, finish string) provider.CompletionResult {
	if finish == provider.FinishStop {
		finish = provider.FinishCanceled
	}
	return provider.CompletionResult{FullText: fullText, FinishReason: finish}
}
