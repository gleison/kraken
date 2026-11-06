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
	planner  Planning
	executor Execution
}

var _ RunService = (*Orchestrator)(nil)

// NewWithPorts composes independently replaceable planning and execution ports.
func NewWithPorts(planner Planning, executor Execution) *Orchestrator {
	return &Orchestrator{planner: planner, executor: executor}
}

func (o *Orchestrator) Plan(ctx context.Context, goal string) <-chan Event {
	out := make(chan Event, 16)

	go func() {
		defer close(out)
		send := eventSender(ctx, out)
		o.forwardToolEvents(send)

		log.Printf("orchestrator: run start (goal_bytes=%d)", len(goal))
		send(Event{Type: EventPlanning})

		plan, err := o.planner.Plan(ctx, goal)
		if err != nil {
			log.Printf("orchestrator: planning failed: %v", err)
			send(Event{Type: EventRunFailed, Err: fmt.Errorf("planning: %w", err)})
			return
		}
		log.Printf("orchestrator: plan ready (%d tasks)", len(plan.Tasks))
		send(Event{Type: EventPlanReady, Plan: plan.Clone()})

		for _, t := range plan.Tasks {
			if ctx.Err() != nil {
				log.Printf("orchestrator: ctx cancelled before task %s: %v", t.ID, ctx.Err())
				send(Event{Type: EventRunFailed, Err: ctx.Err()})
				return
			}

			t.MarkRunning()
			send(Event{Type: EventTaskStarted, Task: t.Clone()})
			result, err := o.executor.Execute(ctx, plan.Goal, t, plan.Tasks)
			if err != nil {
				t.MarkFailed(err)
				send(Event{Type: EventTaskFailed, Task: t.Clone(), Err: err})
				send(Event{Type: EventRunFailed, Err: err})
				return
			}
			t.MarkDone(result)
			send(Event{Type: EventTaskCompleted, Task: t.Clone()})
		}

		log.Printf("orchestrator: run complete")
		send(Event{Type: EventRunCompleted, Plan: plan.Clone(), Final: summarize(plan)})
	}()

	return out
}

func (o *Orchestrator) Chat(ctx context.Context, prompt string, history []domain.Turn) <-chan Event {
	out := make(chan Event, 16)

	go func() {
		defer close(out)
		send := eventSender(ctx, out)
		o.forwardToolEvents(send)

		send(Event{Type: EventChatStarted})
		messages := make([]llm.Message, 0, len(history)*2+1)
		for _, turn := range history {
			messages = append(messages,
				llm.Message{Role: llm.RoleUser, Content: turn.UserInput},
				llm.Message{Role: llm.RoleAssistant, Content: turn.Result},
			)
		}
		messages = append(messages, llm.Message{Role: llm.RoleUser, Content: prompt})

		result, err := o.executor.Chat(ctx, messages)
		if err != nil {
			send(Event{Type: EventRunFailed, Err: err})
			return
		}
		send(Event{Type: EventRunCompleted, Final: result})
	}()

	return out
}

func eventSender(ctx context.Context, out chan<- Event) func(Event) {
	return func(event Event) {
		select {
		case out <- event:
			return
		default:
		}
		select {
		case out <- event:
		case <-ctx.Done():
			select {
			case out <- event:
			default:
			}
		}
	}
}

func (o *Orchestrator) forwardToolEvents(send func(Event)) {
	o.executor.OnToolEvent(func(toolEvent ToolEvent) {
		send(Event{Type: EventToolCall, Tool: &ToolActivity{
			TaskID:    toolEvent.TaskID,
			Iteration: toolEvent.Iteration,
			Call:      toolEvent.Call,
			Result:    toolEvent.Result,
		}})
	})
}

func summarize(plan *domain.Plan) string {
	if plan == nil || len(plan.Tasks) == 0 {
		return ""
	}
	last := plan.Tasks[len(plan.Tasks)-1]
	return strings.TrimSpace(last.Result)
}
