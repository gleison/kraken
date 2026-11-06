package orchestrator

import (
	"github.com/gleison/kraken/internal/domain"
	"github.com/gleison/kraken/internal/tools"
)

// EventType enumerates the lifecycle events emitted during a run.
type EventType string

const (
	EventPlanning      EventType = "planning"
	EventPlanReady     EventType = "plan_ready"
	EventTaskStarted   EventType = "task_started"
	EventTaskCompleted EventType = "task_completed"
	EventTaskFailed    EventType = "task_failed"
	EventToolCall      EventType = "tool_call"
	EventRunCompleted  EventType = "run_completed"
	EventRunFailed     EventType = "run_failed"
)

// Event is the message the orchestrator publishes to observers (e.g. the TUI).
type Event struct {
	Type  EventType
	Plan  *domain.Plan
	Task  *domain.Task
	Err   error
	Final string
	// Tool is populated for EventToolCall: which tool was invoked, with
	// what arguments, and the result (or error) from running it.
	Tool *ToolActivity
}

// ToolActivity is a single tool round, surfaced to the TUI so it can show
// "reading X" / "wrote Y" without coupling to the LLM transport.
type ToolActivity struct {
	TaskID    string
	Iteration int
	Call      tools.Call
	Result    tools.Result
}
