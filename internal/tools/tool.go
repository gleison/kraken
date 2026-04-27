// Package tools defines the capabilities the orchestrator can grant to the
// LLM during a run. Each Tool exposes a JSON Schema for its parameters and
// an executor that runs it locally; the orchestrator forwards the schemas
// to the LLM (via the OpenAI tools API) and routes the model's tool_calls
// back to the corresponding Run.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

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

// Registry holds the tools available for a run.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry builds a Registry from the given tools. Names must be unique.
func NewRegistry(tools ...Tool) *Registry {
	m := make(map[string]Tool, len(tools))
	for _, t := range tools {
		m[t.Name()] = t
	}
	return &Registry{tools: m}
}

// List returns all registered tools in insertion-stable order is not
// guaranteed (callers that care should sort by Name).
func (r *Registry) List() []Tool {
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}

// Has reports whether a tool with the given name is registered.
func (r *Registry) Has(name string) bool {
	_, ok := r.tools[name]
	return ok
}

// Empty reports whether the registry has no tools at all.
func (r *Registry) Empty() bool {
	return len(r.tools) == 0
}

// Invoke decodes the call's raw arguments (if needed) and executes the
// matching tool, returning the populated Result.
func (r *Registry) Invoke(ctx context.Context, c Call) Result {
	res := Result{Call: c}
	t, ok := r.tools[c.Name]
	if !ok {
		res.Err = fmt.Errorf("unknown tool %q", c.Name)
		return res
	}
	args := c.Args
	if args == nil && c.Raw != "" {
		args = map[string]any{}
		if err := json.Unmarshal([]byte(c.Raw), &args); err != nil {
			res.Err = fmt.Errorf("decode args: %w", err)
			return res
		}
	}
	out, err := t.Run(ctx, args)
	res.Content = out
	res.Err = err
	return res
}
