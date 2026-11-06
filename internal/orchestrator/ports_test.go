package orchestrator

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/gleison/kraken/internal/domain"
	"github.com/gleison/kraken/internal/llm"
	"github.com/gleison/kraken/internal/tools"
)

type planningFunc func(context.Context, string) (*domain.Plan, error)

func (f planningFunc) Plan(ctx context.Context, goal string) (*domain.Plan, error) {
	return f(ctx, goal)
}

type executionStub struct {
	execute  func(context.Context, string, *domain.Task, []*domain.Task) (string, error)
	chat     func(context.Context, []llm.Message) (string, error)
	observer func(ToolEvent)
}

func (e *executionStub) Execute(ctx context.Context, goal string, task *domain.Task, previous []*domain.Task) (string, error) {
	return e.execute(ctx, goal, task, previous)
}
func (e *executionStub) Chat(ctx context.Context, messages []llm.Message) (string, error) {
	return e.chat(ctx, messages)
}
func (e *executionStub) OnToolEvent(fn func(ToolEvent)) { e.observer = fn }

func TestPlanWithAlternatePorts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	planner := planningFunc(func(got context.Context, goal string) (*domain.Plan, error) {
		if got != ctx || goal != "goal" {
			t.Error("planning input not forwarded")
		}
		return domain.NewPlan(goal, []*domain.Task{domain.NewTask("1", "task", "output")}), nil
	})
	executor := &executionStub{execute: func(got context.Context, goal string, task *domain.Task, previous []*domain.Task) (string, error) {
		if got != ctx || goal != "goal" || task.Status != domain.StatusRunning || len(previous) != 1 {
			t.Error("execution input not forwarded")
		}
		return "result", nil
	}}
	var types []EventType
	var final Event
	for event := range NewWithPorts(planner, executor).Plan(ctx, "goal") {
		types = append(types, event.Type)
		final = event
	}
	want := []EventType{EventPlanning, EventPlanReady, EventTaskStarted, EventTaskCompleted, EventRunCompleted}
	if !reflect.DeepEqual(types, want) || final.Final != "result" {
		t.Fatalf("events=%v final=%#v", types, final)
	}
}

func TestPlanningPortFailureDoesNotExecute(t *testing.T) {
	failure := errors.New("planning unavailable")
	planner := planningFunc(func(context.Context, string) (*domain.Plan, error) { return nil, failure })
	executor := &executionStub{execute: func(context.Context, string, *domain.Task, []*domain.Task) (string, error) {
		t.Error("execution must not follow a planning failure")
		return "", nil
	}}
	var final Event
	for event := range NewWithPorts(planner, executor).Plan(context.Background(), "goal") {
		final = event
	}
	if final.Type != EventRunFailed || !errors.Is(final.Err, failure) {
		t.Fatalf("final=%#v", final)
	}
}

func TestChatWithAlternatePortsForwardsHistoryAndToolEvents(t *testing.T) {
	planner := planningFunc(func(context.Context, string) (*domain.Plan, error) { t.Error("chat must not plan"); return nil, nil })
	executor := &executionStub{}
	executor.chat = func(_ context.Context, messages []llm.Message) (string, error) {
		want := []llm.Message{{Role: llm.RoleUser, Content: "question"}, {Role: llm.RoleAssistant, Content: "answer"}, {Role: llm.RoleUser, Content: "next"}}
		if !reflect.DeepEqual(messages, want) {
			t.Errorf("messages=%#v", messages)
		}
		executor.observer(ToolEvent{TaskID: "chat", Iteration: 1, Call: tools.Call{Name: "alternate"}})
		return "done", nil
	}
	var events []Event
	for event := range NewWithPorts(planner, executor).Chat(context.Background(), "next", []domain.Turn{{UserInput: "question", Result: "answer"}}) {
		events = append(events, event)
	}
	if len(events) != 3 || events[0].Type != EventChatStarted || events[1].Type != EventToolCall || events[1].Tool.Call.Name != "alternate" || events[2].Final != "done" {
		t.Fatalf("events=%#v", events)
	}
}

// This catalog has no Registry dependency; the executor must advertise and
// invoke its tools solely through the port, then return the result to the LLM.
type alternateCatalog struct{ calls []tools.Call }

func (*alternateCatalog) List() []tools.Tool { return []tools.Tool{&tools.ReadFile{}} }
func (*alternateCatalog) Empty() bool        { return false }
func (c *alternateCatalog) Invoke(_ context.Context, call tools.Call) tools.Result {
	c.calls = append(c.calls, call)
	return tools.Result{Call: call, Content: "fixture content"}
}

func TestExecutorWithAlternateToolCatalog(t *testing.T) {
	catalog := &alternateCatalog{}
	round := 0
	client := clientFunc(func(_ context.Context, req llm.Request) (*llm.Response, error) {
		round++
		if len(req.Tools) != 1 || req.Tools[0].Name != "read_file" {
			t.Errorf("tools=%#v", req.Tools)
		}
		if round == 1 {
			return &llm.Response{ToolCalls: []llm.ToolCall{{ID: "call", Name: "read_file", Args: `{"path":"file.txt"}`}}}, nil
		}
		last := req.Messages[len(req.Messages)-1]
		if last.Role != llm.RoleTool || last.ToolCallID != "call" || last.Content != "fixture content" {
			t.Errorf("tool reply=%#v", last)
		}
		return &llm.Response{Content: "done"}, nil
	})
	result, err := NewExecutor(client).WithTools(catalog).Chat(context.Background(), []llm.Message{{Role: llm.RoleUser, Content: "read"}})
	if err != nil || result != "done" || round != 2 || len(catalog.calls) != 1 || catalog.calls[0].Raw != `{"path":"file.txt"}` {
		t.Fatalf("result=%q err=%v rounds=%d calls=%#v", result, err, round, catalog.calls)
	}
}
