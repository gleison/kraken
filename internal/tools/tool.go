// Package tools defines the capabilities the orchestrator can grant to the
// LLM during a run. Each Tool exposes a JSON Schema for its parameters and
// an executor that runs it locally; the orchestrator forwards the schemas
// to the LLM (via the OpenAI tools API) and routes the model's tool_calls
// back to the corresponding Run.
package tools

import "context"

// Tool is a single capability the orchestrator can grant the LLM.
type Tool interface {
	Name() string
	Description() string
	// Schema returns a JSON Schema describing the parameters the LLM must
	// supply. Anything that marshals to a valid schema is acceptable
	// (map[string]any, custom struct, json.RawMessage).
	Schema() any
	// Run executes the tool with the LLM-supplied arguments and returns
	// the textual result that will be fed back to the model.
	Run(ctx context.Context, args map[string]any) (string, error)
}

// Call is what the LLM asked the orchestrator to do.
type Call struct {
	ID   string         // provider-supplied call id
	Name string         // tool name
	Args map[string]any // decoded arguments
	Raw  string         // raw JSON arguments (kept for logging/replay)
}

// Result is the outcome of executing a Call.
type Result struct {
	Call    Call
	Content string
	Err     error
}

// Catalog is the tool port used by the executor. Implementations advertise
// available capabilities and dispatch calls without exposing their storage.
type Catalog interface {
	List() []Tool
	Empty() bool
	Invoke(ctx context.Context, call Call) Result
}
