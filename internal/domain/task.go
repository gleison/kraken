// Package domain holds the core entities of the orchestrator.
// It has no dependencies on other layers.
package domain

import "time"

// Status represents the lifecycle state of a Task.
type Status string

const (
	StatusPending Status = "pending"
	StatusRunning Status = "running"
	StatusDone    Status = "done"
	StatusFailed  Status = "failed"
)

// Task is the smallest unit of work in the orchestration flow:
// a single, simple and direct instruction to be fulfilled by an LLM.
type Task struct {
	ID          string
	Title       string
	Instruction string
	Result      string
	Status      Status
	Err         string
	StartedAt   time.Time
	FinishedAt  time.Time
}

func NewTask(id, title, instruction string) *Task {
	return &Task{
		ID:          id,
		Title:       title,
		Instruction: instruction,
		Status:      StatusPending,
	}
}

func (t *Task) Clone() *Task {
	if t == nil {
		return nil
	}
	copy := *t
	return &copy
}

func (t *Task) MarkRunning() {
	t.Status = StatusRunning
	t.StartedAt = time.Now()
}

func (t *Task) MarkDone(result string) {
	t.Result = result
	t.Status = StatusDone
	t.FinishedAt = time.Now()
}

func (t *Task) MarkFailed(err error) {
	if err != nil {
		t.Err = err.Error()
	}
	t.Status = StatusFailed
	t.FinishedAt = time.Now()
}
