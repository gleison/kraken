package orchestrator

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gleison/kraken/internal/domain"
	"github.com/gleison/kraken/internal/llm"
	"github.com/gleison/kraken/internal/tools"
)

const executorSystem = `You are an executor agent in a multi-step pipeline.
You receive ONE atomic task and the results of previous tasks.
Respond with a concise, direct answer that fulfills ONLY the current task.
Do not restate the task. Do not add meta-commentary.
You have file-system tools available; use them when the task requires reading or writing files. Never invent file contents — call read_file first.`

// maxToolIterations bounds how many tool-call rounds a single task may
// trigger. A misbehaving model could otherwise loop forever calling the
// same tool.
const maxToolIterations = 8

// ToolEvent is emitted by the Executor whenever a tool round occurs, so
// the UI can show "reading X" / "wrote Y" without coupling to the LLM
// transport.
type ToolEvent struct {
	TaskID   string
	Call     tools.Call
	Result   tools.Result
	Iteration int
}

// Executor runs a single task against the LLM. When a Registry is set,
// the model may call tools; the executor loops until the model stops
// requesting them (or until maxToolIterations is reached).
type Executor struct {
	client   llm.Client
	registry *tools.Registry
	onTool   func(ToolEvent)
}

// NewExecutor wires the Executor with its LLM dependency.
func NewExecutor(c llm.Client) *Executor {
	return &Executor{client: c}
}

// WithTools attaches a tool registry. nil or empty registry leaves the
// executor in plain text-in/text-out mode.
func (e *Executor) WithTools(r *tools.Registry) *Executor {
	e.registry = r
	return e
}

// OnToolEvent installs a callback invoked synchronously for each tool
// round. Used by the Orchestrator to forward the activity to the TUI.
func (e *Executor) OnToolEvent(fn func(ToolEvent)) *Executor {
	e.onTool = fn
	return e
}

// Execute runs a task and mutates its state. The caller supplies the already
// completed tasks so their results can be passed as context.
func (e *Executor) Execute(ctx context.Context, goal string, task *domain.Task, previous []*domain.Task) error {
	task.MarkRunning()

	prompt := buildExecutorPrompt(goal, task, previous)
	log.Printf("executor: task %s start (prompt_bytes=%d, tools=%t)",
		task.ID, len(prompt), e.toolsAvailable())
	start := time.Now()

	messages := []llm.Message{{Role: llm.RoleUser, Content: prompt}}
	tools := e.requestTools()

	for iter := 0; iter < maxToolIterations; iter++ {
		resp, err := e.client.Complete(ctx, llm.Request{
			System:   executorSystem,
			Messages: messages,
			Tools:    tools,
		})
		if err != nil {
			log.Printf("executor: task %s failed in %s: %v", task.ID, time.Since(start), err)
			task.MarkFailed(err)
			return fmt.Errorf("executor: %w", err)
		}

		if len(resp.ToolCalls) == 0 {
			content := strings.TrimSpace(resp.Content)
			log.Printf("executor: task %s done in %s (result_bytes=%d, iters=%d)",
				task.ID, time.Since(start), len(content), iter+1)
			task.MarkDone(content)
			return nil
		}

		// Echo the assistant's tool_calls back so the next request can
		// correlate the tool results.
		messages = append(messages, llm.Message{
			Role:      llm.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		for _, tc := range resp.ToolCalls {
			result := e.runTool(ctx, task.ID, iter+1, tc)
			messages = append(messages, llm.Message{
				Role:       llm.RoleTool,
				Content:    formatToolMessage(result),
				ToolCallID: tc.ID,
			})
		}
	}

	err := fmt.Errorf("executor: task %s exceeded %d tool iterations", task.ID, maxToolIterations)
	log.Printf("%v", err)
	task.MarkFailed(err)
	return err
}

func (e *Executor) toolsAvailable() bool {
	return e.registry != nil && !e.registry.Empty()
}

// requestTools projects the registry to the wire format the LLM client
// expects. Returns nil when no tools are registered, so we don't even
// send a `tools` field on the request.
func (e *Executor) requestTools() []llm.Tool {
	if !e.toolsAvailable() {
		return nil
	}
	list := e.registry.List()
	out := make([]llm.Tool, 0, len(list))
	for _, t := range list {
		out = append(out, llm.Tool{
			Name:        t.Name(),
			Description: t.Description(),
			Schema:      t.Schema(),
		})
	}
	return out
}

func (e *Executor) runTool(ctx context.Context, taskID string, iter int, tc llm.ToolCall) tools.Result {
	call := tools.Call{ID: tc.ID, Name: tc.Name, Raw: tc.Args}
	if e.registry == nil {
		return tools.Result{Call: call, Err: fmt.Errorf("no tools registered")}
	}
	log.Printf("tools: invoke task=%s iter=%d name=%s args=%s",
		taskID, iter, tc.Name, truncate(tc.Args, 200))
	result := e.registry.Invoke(ctx, call)
	if result.Err != nil {
		log.Printf("tools: task=%s name=%s failed: %v", taskID, tc.Name, result.Err)
	} else {
		log.Printf("tools: task=%s name=%s ok (%d bytes)", taskID, tc.Name, len(result.Content))
	}
	if e.onTool != nil {
		e.onTool(ToolEvent{TaskID: taskID, Call: call, Result: result, Iteration: iter})
	}
	return result
}

// formatToolMessage builds the content string the LLM sees in the
// role:"tool" reply. Errors are surfaced explicitly so the model can
// adjust (try a different path, fall back) rather than silently produce
// a hallucinated answer.
func formatToolMessage(r tools.Result) string {
	if r.Err != nil {
		return fmt.Sprintf("error: %s", r.Err.Error())
	}
	return r.Content
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
