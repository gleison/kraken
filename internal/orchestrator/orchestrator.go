package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/gleison/kraken/internal/domain"
	"github.com/gleison/kraken/internal/llm"
)

// Orchestrator composes a Planner and an Executor to turn a goal into
// a sequence of task executions, publishing lifecycle Events on a channel.
type Orchestrator struct {
	planner  *Planner
	executor *Executor
}

// New builds an Orchestrator from an LLM client.
func New(client llm.Client) *Orchestrator {
	return &Orchestrator{
		planner:  NewPlanner(client),
		executor: NewExecutor(client),
	}
}

// Run executes the full flow and returns a read-only event channel.
// The channel is closed when the run ends (success or failure).
func (o *Orchestrator) Run(ctx context.Context, goal string) <-chan Event {
	out := make(chan Event, 8)

	go func() {
		defer close(out)

		send := func(e Event) {
			select {
			case out <- e:
			case <-ctx.Done():
			}
		}

		send(Event{Type: EventPlanning})

		plan, err := o.planner.Plan(ctx, goal)
		if err != nil {
			send(Event{Type: EventRunFailed, Err: fmt.Errorf("planning: %w", err)})
			return
		}
		send(Event{Type: EventPlanReady, Plan: plan})

		for _, t := range plan.Tasks {
			if ctx.Err() != nil {
				send(Event{Type: EventRunFailed, Err: ctx.Err()})
				return
			}

			send(Event{Type: EventTaskStarted, Task: t})
			if err := o.executor.Execute(ctx, plan.Goal, t, plan.Tasks); err != nil {
				send(Event{Type: EventTaskFailed, Task: t, Err: err})
				send(Event{Type: EventRunFailed, Err: err})
				return
			}
			send(Event{Type: EventTaskCompleted, Task: t})
		}

		send(Event{Type: EventRunCompleted, Plan: plan, Final: summarize(plan)})
	}()

	return out
}

func summarize(plan *domain.Plan) string {
	if plan == nil || len(plan.Tasks) == 0 {
		return ""
	}
	last := plan.Tasks[len(plan.Tasks)-1]
	return strings.TrimSpace(last.Result)
}
