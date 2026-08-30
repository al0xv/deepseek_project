// Package mock provides a fake provider.Provider for development and tests.
// It never calls an external API.
package mock

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"deepseek-terminal/internal/provider"
)

// Provider echoes the last user message back, one character at a time, so
// streaming can be observed without any API key.
type Provider struct {
	Delay time.Duration

	mu      sync.Mutex
	lastReq provider.CompletionRequest
}

// LastRequest returns the most recent completion request (for tests).
func (p *Provider) LastRequest() provider.CompletionRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastReq
}

// StreamCompletion implements provider.Provider.
func (p *Provider) StreamCompletion(ctx context.Context, req provider.CompletionRequest, onDelta func(delta string) error) (provider.CompletionResult, error) {
	p.mu.Lock()
	p.lastReq = req
	p.mu.Unlock()

	last := ""
	if n := len(req.Messages); n > 0 {
		last = req.Messages[n-1].Content
	}
	reply := fmt.Sprintf("mock reply to: %s\n", last)

	var sb strings.Builder
	for _, ch := range reply {
		if p.Delay > 0 {
			select {
			case <-time.After(p.Delay):
			case <-ctx.Done():
				return provider.CompletionResult{FullText: sb.String(), FinishReason: provider.FinishCanceled}, nil
			}
		}
		if onDelta != nil {
			if err := onDelta(string(ch)); err != nil {
				return provider.CompletionResult{}, err
			}
		}
		sb.WriteRune(ch)
	}
	return provider.CompletionResult{FullText: sb.String(), FinishReason: provider.FinishStop}, nil
}
