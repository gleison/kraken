package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gleison/kraken/internal/domain"
	"github.com/gleison/kraken/internal/llm"
)

const plannerSystem = `You are a planner. Your job is to break a complex goal into a short, ordered list of simple, direct tasks that an LLM executor can perform, each in one focused step.

Rules:
- Produce between 2 and 6 tasks.
- Each task must be atomic, unambiguous and independently executable given the previous results.
- Return ONLY valid JSON, no prose, with the shape:
  {"tasks":[{"title":"...","instruction":"..."}]}
- Write tasks in the same language as the user goal.`

// Planner decomposes a goal into a Plan via an LLM call.
type Planner struct {
	client llm.Client
}

// NewPlanner wires the Planner with its LLM dependency.
func NewPlanner(c llm.Client) *Planner {
	return &Planner{client: c}
}

type plannerPayload struct {
	Tasks []struct {
		Title       string `json:"title"`
		Instruction string `json:"instruction"`
	} `json:"tasks"`
}

// Plan asks the LLM to decompose the goal and returns the resulting Plan.
func (p *Planner) Plan(ctx context.Context, goal string) (*domain.Plan, error) {
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return nil, fmt.Errorf("planner: empty goal")
	}

	resp, err := p.client.Complete(ctx, llm.Request{
		System: plannerSystem,
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: goal},
		},
		MaxTokens: 1024,
		JSONMode:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("planner: llm call: %w", err)
	}

	payload, err := parsePlannerPayload(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("planner: parse response: %w (raw=%s)", err, snippet(resp.Content, 200))
	}
	if len(payload.Tasks) == 0 {
		return nil, fmt.Errorf("planner: no tasks returned")
	}

	tasks := make([]*domain.Task, 0, len(payload.Tasks))
	for i, t := range payload.Tasks {
		title := strings.TrimSpace(t.Title)
		instr := strings.TrimSpace(t.Instruction)
		if title == "" {
			title = "Task " + strconv.Itoa(i+1)
		}
		if instr == "" {
			continue
		}
		tasks = append(tasks, domain.NewTask(strconv.Itoa(i+1), title, instr))
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("planner: all tasks were empty")
	}
	return domain.NewPlan(goal, tasks), nil
}

// parsePlannerPayload extracts JSON even if the model wrapped it in prose/fences.
func parsePlannerPayload(raw string) (*plannerPayload, error) {
	s := extractJSONObject(raw)
	if s == "" {
		return nil, fmt.Errorf("no JSON object found in response")
	}
	var p plannerPayload
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// extractJSONObject returns the first balanced {...} block in s, or "".
func extractJSONObject(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}
	depth := 0
	inStr := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		switch {
		case escaped:
			escaped = false
		case c == '\\' && inStr:
			escaped = true
		case c == '"':
			inStr = !inStr
		case inStr:
			// skip
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

// snippet returns a single-line, length-capped preview of s for use in
// error messages when parsing fails.
func snippet(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
