// Package llm defines the abstraction used by the orchestrator
// to talk to a Large Language Model provider.
package llm

import "context"

// Client is the port the orchestrator depends on.
// Any provider (OpenAI, Azure, local server, etc.) implements it.
type Client interface {
	Complete(ctx context.Context, req Request) (*Response, error)
}
