package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/gleison/kraken/internal/llm"
)

type clientFunc func(context.Context, llm.Request) (*llm.Response, error)

func (f clientFunc) Complete(ctx context.Context, req llm.Request) (*llm.Response, error) {
	return f(ctx, req)
}

func TestParsePlannerPayloadRepairsModelJSON(t *testing.T) {
	raw := "prefix ```json\n{\"tasks\":[{\"title\":\"one\",\"instruction\":\"line 1\nline 2\\'\"}]}\n``` suffix"
	payload, err := parsePlannerPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload.Tasks) != 1 || payload.Tasks[0].Instruction != "line 1\nline 2'" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestExtractJSONObjectIgnoresBracesInsideStrings(t *testing.T) {
	got := extractJSONObject(`before {"value":"}"} after {"ignored":true}`)
	if got != `{"value":"}"}` {
		t.Fatalf("got %q", got)
	}
}

func TestPlannerAnnouncesTools(t *testing.T) {
	var request llm.Request
	client := clientFunc(func(_ context.Context, req llm.Request) (*llm.Response, error) {
		request = req
		return &llm.Response{Content: `{"tasks":[{"title":"inspect","instruction":"read"},{"title":"deliver","instruction":"output"}]}`}, nil
	})
	planner := NewPlanner(client)
	planner.AnnounceTools([]string{"read_file — read a file"})

	plan, err := planner.Plan(context.Background(), "do work")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Tasks) != 2 {
		t.Fatalf("got %d tasks", len(plan.Tasks))
	}
	if !strings.Contains(request.System, "read_file — read a file") {
		t.Fatalf("tool missing from system prompt: %q", request.System)
	}
	if request.JSONSchema == nil {
		t.Fatal("planner did not request structured output")
	}
}
