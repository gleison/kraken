// Package llm defines the abstraction used by the orchestrator
// to talk to a Large Language Model provider.
package llm

import "context"

type Client interface {
	Complete(ctx context.Context, req Request) (*Response, error)
}
