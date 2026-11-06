package orchestrator

import (
	"context"

	"github.com/gleison/kraken/internal/domain"
	"github.com/gleison/kraken/internal/llm"
	"github.com/gleison/kraken/internal/tools"
)

type Planning interface {
	Plan(ctx context.Context, goal string) (*domain.Plan, error)
}

// Execution runs tasks and chat turns. The orchestrator installs a synchronous
// tool observer before each run. A single instance serves one run at a time.
type Execution interface {
	Execute(ctx context.Context, goal string, task *domain.Task, previous []*domain.Task) (string, error)
	Chat(ctx context.Context, messages []llm.Message) (string, error)
	OnToolEvent(func(ToolEvent))
}

// RunService is the application port consumed by presentation adapters.
// Implementations close the event channel when the run ends and honor context
// cancellation. Plan and task events carry snapshots owned by the consumer.
// This is not the future sandbox command runner.
type RunService interface {
	Plan(ctx context.Context, goal string) <-chan Event
	Chat(ctx context.Context, prompt string, history []domain.Turn) <-chan Event
}

type ToolEvent struct {
	TaskID    string
	Call      tools.Call
	Result    tools.Result
	Iteration int
}
