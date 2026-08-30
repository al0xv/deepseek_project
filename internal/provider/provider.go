// Package provider defines the minimal abstraction over chat completion APIs.
package provider

import "context"

// Role constants used in chat messages.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// Message is a single chat message in a conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Finish reason constants returned by providers.
const (
	FinishStop     = "stop"
	FinishLength   = "length"
	FinishCanceled = "cancelled"
)

// CompletionRequest describes one completion call.
type CompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// CompletionResult is the accumulated text of a streamed completion.
type CompletionResult struct {
	FullText     string `json:"full_text"`
	FinishReason string `json:"finish_reason"`
}

// Provider streams a completion for a conversation.
//
// onDelta is called for each text delta as it arrives. If onDelta returns an
// error, streaming stops and that error is returned.
//
// When ctx is cancelled the provider returns whatever text was accumulated so
// far with FinishReason "cancelled" and a nil error.
type Provider interface {
	StreamCompletion(ctx context.Context, req CompletionRequest, onDelta func(delta string) error) (CompletionResult, error)
}
