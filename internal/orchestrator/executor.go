package orchestrator

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gleison/kraken/internal/domain"
	"github.com/gleison/kraken/internal/llm"
)

const executorSystem = `You are an executor agent in a multi-step pipeline.
You receive ONE atomic task and the results of previous tasks.
Respond with a concise, direct answer that fulfills ONLY the current task.
Do not restate the task. Do not add meta-commentary.`

// Executor runs a single task against the LLM, given the accumulated context
// of previous task results.
type Executor struct {
	client llm.Client
}

// NewExecutor wires the Executor with its LLM dependency.
func NewExecutor(c llm.Client) *Executor {
	return &Executor{client: c}
}

// Execute runs a task and mutates its state. The caller supplies the already
// completed tasks so their results can be passed as context.
func (e *Executor) Execute(ctx context.Context, goal string, task *domain.Task, previous []*domain.Task) error {
	task.MarkRunning()

	prompt := buildExecutorPrompt(goal, task, previous)
	log.Printf("executor: task %s start (prompt_bytes=%d)", task.ID, len(prompt))
	start := time.Now()

	resp, err := e.client.Complete(ctx, llm.Request{
		System:   executorSystem,
		Messages: []llm.Message{{Role: llm.RoleUser, Content: prompt}},
		// MaxTokens left at 0 → adapter falls back to its instance
		// default (Config.MaxTokens / OPENAI_MAX_TOKENS), so code
		// generation isn't capped at the planner's smaller budget.
	})
	if err != nil {
		log.Printf("executor: task %s failed in %s: %v", task.ID, time.Since(start), err)
		task.MarkFailed(err)
		return fmt.Errorf("executor: %w", err)
	}

	log.Printf("executor: task %s done in %s (result_bytes=%d)",
		task.ID, time.Since(start), len(resp.Content))
	task.MarkDone(strings.TrimSpace(resp.Content))
	return nil
}

func buildExecutorPrompt(goal string, task *domain.Task, previous []*domain.Task) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Overall goal: %s\n\n", goal)

	if len(previous) > 0 {
		b.WriteString("Previous task results:\n")
		for _, p := range previous {
			if p.Status != domain.StatusDone {
				continue
			}
			fmt.Fprintf(&b, "- [%s] %s\n  %s\n", p.ID, p.Title, p.Result)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "Current task: %s\n", task.Title)
	fmt.Fprintf(&b, "Instruction: %s\n", task.Instruction)
	return b.String()
}
