package orchestrator

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/gleison/kraken/internal/domain"
	"github.com/gleison/kraken/internal/llm"
)

func TestOrchestratorEmitsImmutableSnapshots(t *testing.T) {
	responses := []*llm.Response{
		{Content: `{"tasks":[{"title":"first","instruction":"do first"},{"title":"second","instruction":"do second"}]}`},
		{Content: "first result"},
		{Content: "final result"},
	}
	var mu sync.Mutex
	client := clientFunc(func(_ context.Context, _ llm.Request) (*llm.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		if len(responses) == 0 {
			t.Fatal("unexpected LLM call")
		}
		response := responses[0]
		responses = responses[1:]
		return response, nil
	})

	var initialPlan *domain.Plan
	var completed []*domain.Task
	var final Event
	for event := range New(client).Run(context.Background(), "goal") {
		switch event.Type {
		case EventPlanReady:
			initialPlan = event.Plan
		case EventTaskCompleted:
			completed = append(completed, event.Task)
		case EventRunCompleted:
			final = event
		}
	}

	if initialPlan == nil || initialPlan.Tasks[0].Status != domain.StatusPending {
		t.Fatalf("initial snapshot was mutated: %#v", initialPlan)
	}
	if len(completed) != 2 || completed[0].Result != "first result" || completed[1].Result != "final result" {
		t.Fatalf("completed snapshots = %#v", completed)
	}
	if final.Final != "final result" || final.Plan.Tasks[1].Status != domain.StatusDone {
		t.Fatalf("final event = %#v", final)
	}
}

func TestOrchestratorPropagatesCancellation(t *testing.T) {
	started := make(chan struct{})
	client := clientFunc(func(ctx context.Context, _ llm.Request) (*llm.Response, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	events := New(client).Run(ctx, "goal")
	<-started
	cancel()

	var runError error
	for event := range events {
		if event.Type == EventRunFailed {
			runError = event.Err
		}
	}
	if !errors.Is(runError, context.Canceled) {
		t.Fatalf("error = %v", runError)
	}
}
