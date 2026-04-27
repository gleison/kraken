package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gleison/kraken/internal/domain"
	"github.com/gleison/kraken/internal/llm"
)

const plannerBaseSystem = `You are a planner. Your job is to break a complex goal into a short, ordered list of simple, direct tasks that an LLM executor can perform, each in one focused step.

Rules:
- Produce between 2 and 6 tasks.
- Each task must be atomic, unambiguous and independently executable given the previous results.
- The LAST task MUST produce the concrete deliverable the user actually asked for (the corrected code, the rewritten text, the final answer). It must say "Output ..." or equivalent — never just "summarize" or "describe".
- When the goal is to modify code: include a task that identifies the exact root cause, then a final task that outputs the FULL corrected code (not a description, not a diff in prose) inside a fenced code block.
- Do not invent or paraphrase the user's input. Preserve their content verbatim when handing it to subsequent tasks.
- Return ONLY valid JSON, no prose, with the shape:
  {"tasks":[{"title":"...","instruction":"..."}]}
- Write tasks in the same language as the user goal.`

const plannerToolHint = `

The executor has these workspace tools available; instruct it to use them when the goal touches files (instead of asking the user to paste contents):
%s
Always tell the executor to call read_file before editing, and to call write_file with the FULL new contents when modifying a file.`

// Planner decomposes a goal into a Plan via an LLM call.
type Planner struct {
	client llm.Client
	// toolHint, when non-empty, is appended to the planner's system
	// prompt so the model knows which workspace tools the executor can
	// use and writes plan steps accordingly.
	toolHint string
}

// NewPlanner wires the Planner with its LLM dependency.
func NewPlanner(c llm.Client) *Planner {
	return &Planner{client: c}
}

// AnnounceTools registers the tools the executor will have available, so
// the planner can write tasks that delegate file work to them. tools is
// a slice of "name — description" snippets (one per tool); pass nil to
// remove a previous announcement.
func (p *Planner) AnnounceTools(tools []string) {
	if len(tools) == 0 {
		p.toolHint = ""
		return
	}
	var b strings.Builder
	for _, t := range tools {
		b.WriteString("- ")
		b.WriteString(t)
		b.WriteString("\n")
	}
	p.toolHint = fmt.Sprintf(plannerToolHint, b.String())
}

// systemPrompt returns the full prompt for the planner, including any
// tool announcement.
func (p *Planner) systemPrompt() string {
	return plannerBaseSystem + p.toolHint
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
		System: p.systemPrompt(),
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: goal},
		},
		MaxTokens:  1024,
		JSONSchema: plannerSchema(),
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
// Small models (Gemma 2B and friends) sometimes emit raw control characters
// inside string values, so we transparently repair those before failing.
func parsePlannerPayload(raw string) (*plannerPayload, error) {
	s := extractJSONObject(raw)
	if s == "" {
		return nil, fmt.Errorf("no JSON object found in response")
	}
	var p plannerPayload
	if err := json.Unmarshal([]byte(s), &p); err == nil {
		log.Printf("planner: parsed %d task(s) on first pass", len(p.Tasks))
		return &p, nil
	} else {
		log.Printf("planner: first parse failed (%v); trying repair", err)
	}
	repaired := repairJSONStrings(s)
	if err := json.Unmarshal([]byte(repaired), &p); err != nil {
		log.Printf("planner: repaired parse also failed: %v", err)
		return nil, err
	}
	log.Printf("planner: parsed %d task(s) after repair", len(p.Tasks))
	return &p, nil
}

// repairJSONStrings makes a best-effort pass to fix string-literal mistakes
// that small models commonly produce: raw control characters inside a
// string, and backslashes followed by something that isn't a valid JSON
// escape (e.g. "\'", "\<LF>"). Outside strings the input is untouched.
//
// Recognised escapes after '\\' (kept verbatim): " \ / b f n r t u.
// Raw LF/CR/TAB inside a string become \n / \r / \t.
// "\\<control>" becomes the canonical escape (\n etc).
// "\\<other>" drops the backslash, keeping the next character.
func repairJSONStrings(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inStr := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !inStr {
			b.WriteByte(c)
			if c == '"' {
				inStr = true
			}
			continue
		}
		switch c {
		case '"':
			b.WriteByte(c)
			inStr = false
		case '\\':
			i = repairEscape(&b, s, i)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// repairEscape handles one '\\' sequence inside a string literal and
// returns the index of the last consumed byte. It assumes s[i] == '\\'.
func repairEscape(b *strings.Builder, s string, i int) int {
	if i+1 >= len(s) {
		b.WriteByte('\\')
		return i
	}
	next := s[i+1]
	switch next {
	case '"', '\\', '/', 'b', 'f', 'n', 'r', 't', 'u':
		b.WriteByte('\\')
		b.WriteByte(next)
	case '\n':
		b.WriteString(`\n`)
	case '\r':
		b.WriteString(`\r`)
	case '\t':
		b.WriteString(`\t`)
	default:
		// Invalid escape — drop the backslash, keep the literal byte.
		b.WriteByte(next)
	}
	return i + 1
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

// plannerSchema returns the JSON Schema describing the Plan structure.
// Providers that honor it (via response_format=json_schema) will produce
// valid JSON; others degrade gracefully to json_object or plain text via
// the OpenAI adapter's cascade.
func plannerSchema() *llm.JSONSchema {
	return &llm.JSONSchema{
		Name:   "plan",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tasks": map[string]any{
					"type":     "array",
					"minItems": 2,
					"maxItems": 6,
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"title":       map[string]any{"type": "string"},
							"instruction": map[string]any{"type": "string"},
						},
						"required":             []string{"title", "instruction"},
						"additionalProperties": false,
					},
				},
			},
			"required":             []string{"tasks"},
			"additionalProperties": false,
		},
	}
}
