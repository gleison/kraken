package orchestrator

import (
	"context"
	"fmt"
	"log"
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

// Planner returns the underlying planner, exposed so callers can adjust
// its prompt based on tooling availability.
func (o *Orchestrator) Planner() *Planner { return o.planner }

// Executor returns the underlying executor, exposed for tool wiring.
func (o *Orchestrator) Executor() *Executor { return o.executor }

// Run executes the full flow and returns a read-only event channel.
// The channel is closed when the run ends (success or failure).
func (o *Orchestrator) Run(ctx context.Context, goal string) <-chan Event {
	out := make(chan Event, 16)

	go func() {
		defer close(out)

		send := func(e Event) {
			select {
			case out <- e:
			case <-ctx.Done():
			}
		}

		// Forward Executor tool activity through the same event stream.
		o.executor.OnToolEvent(func(te ToolEvent) {
			send(Event{Type: EventToolCall, Tool: &ToolActivity{
				TaskID:    te.TaskID,
				Iteration: te.Iteration,
				Call:      te.Call,
				Result:    te.Result,
			}})
		})

		log.Printf("orchestrator: run start (goal_bytes=%d)", len(goal))
		send(Event{Type: EventPlanning})

		plan, err := o.planner.Plan(ctx, goal)
		if err != nil {
			log.Printf("orchestrator: planning failed: %v", err)
			send(Event{Type: EventRunFailed, Err: fmt.Errorf("planning: %w", err)})
			return
		}
		log.Printf("orchestrator: plan ready (%d tasks)", len(plan.Tasks))
		send(Event{Type: EventPlanReady, Plan: plan})

		for _, t := range plan.Tasks {
			if ctx.Err() != nil {
				log.Printf("orchestrator: ctx cancelled before task %s: %v", t.ID, ctx.Err())
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

		log.Printf("orchestrator: run complete")
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
