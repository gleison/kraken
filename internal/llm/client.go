// Package llm defines the abstraction used by the orchestrator
// to talk to a Large Language Model provider.
package llm

import "context"

// Role is the author of a message in a chat-style request.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is a single turn in the conversation.
//
// For RoleTool messages, ToolCallID identifies which tool_call produced
// this result and Content carries the result text. For RoleAssistant
// messages emitted from a previous turn, ToolCalls echoes the calls the
// model made so providers can correlate them on the next round.
type Message struct {
	Role       Role
	Content    string
	ToolCalls  []ToolCall // assistant messages that requested tools
	ToolCallID string     // tool messages: id of the call being answered
}

// JSONSchema describes a Structured Output contract that the provider
// must honor. When set, the model's response is constrained to match Schema.
// Schema can be any value that marshals to a valid JSON Schema (map, struct,
// json.RawMessage, etc.).
type JSONSchema struct {
	Name   string
	Strict bool
	Schema any
}

// Tool describes a function the model may invoke during the call.
// Schema is the JSON Schema for the tool's arguments.
type Tool struct {
	Name        string
	Description string
	Schema      any
}

// ToolCall is a single function invocation requested by the model.
// Args is the raw JSON string the model produced; the orchestrator
// decodes it before dispatching to the matching tool.
type ToolCall struct {
	ID   string
	Name string
	Args string
}

// Request is a provider-agnostic completion request.
type Request struct {
	System     string
	Messages   []Message
	MaxTokens  int
	JSONSchema *JSONSchema
	Tools      []Tool
}

// Response carries the text returned by the provider, plus any tool
// calls the model wants to make. When ToolCalls is non-empty the
// orchestrator should execute them and call Complete again with the
// results appended as RoleTool messages.
type Response struct {
	Content   string
	ToolCalls []ToolCall
}

// Client is the port the orchestrator depends on.
// Any provider (OpenAI, Azure, local server, etc.) implements it.
type Client interface {
	Complete(ctx context.Context, req Request) (*Response, error)
}
