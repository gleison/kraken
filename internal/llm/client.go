// Package llm defines the abstraction used by the orchestrator
// to talk to a Large Language Model provider.
package llm

import "context"

// Role is the author of a message in a chat-style request.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is a single turn in the conversation.
type Message struct {
	Role    Role
	Content string
}

// Request is a provider-agnostic completion request.
type Request struct {
	System    string
	Messages  []Message
	MaxTokens int
}

// Response carries the text returned by the provider.
type Response struct {
	Content string
}

// Client is the port the orchestrator depends on.
// Any provider (Anthropic, OpenAI, local model, mock) implements it.
type Client interface {
	Complete(ctx context.Context, req Request) (*Response, error)
}
