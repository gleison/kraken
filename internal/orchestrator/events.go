package orchestrator

import "github.com/gleison/kraken/internal/domain"

// EventType enumerates the lifecycle events emitted during a run.
type EventType string

const (
	EventPlanning       EventType = "planning"
	EventPlanReady      EventType = "plan_ready"
	EventTaskStarted    EventType = "task_started"
	EventTaskCompleted  EventType = "task_completed"
	EventTaskFailed     EventType = "task_failed"
	EventRunCompleted   EventType = "run_completed"
	EventRunFailed      EventType = "run_failed"
)

// Event is the message the orchestrator publishes to observers (e.g. the TUI).
type Event struct {
	Type  EventType
	Plan  *domain.Plan
	Task  *domain.Task
	Err   error
	Final string
}
