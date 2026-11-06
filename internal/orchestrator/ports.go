package orchestrator

import (
	"context"

	"github.com/gleison/kraken/internal/domain"
	"github.com/gleison/kraken/internal/llm"
	"github.com/gleison/kraken/internal/tools"
)

// Planning decomposes a goal. Tool announcements are configured on the
// concrete planner at composition time, outside this runtime contract.
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

// ToolEvent is emitted by the Executor whenever a tool round occurs, so
// the UI can show "reading X" / "wrote Y" without coupling to the LLM
// transport.
type ToolEvent struct {
	TaskID    string
	Call      tools.Call
	Result    tools.Result
	Iteration int
}
