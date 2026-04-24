package llm

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Mock is a deterministic Client used when no API key is configured.
// It lets the TUI be demoed end-to-end without real network calls.
type Mock struct {
	Latency time.Duration
}

// NewMock returns a Mock with a small artificial latency.
func NewMock() *Mock {
	return &Mock{Latency: 400 * time.Millisecond}
}

// Complete implements llm.Client.
func (m *Mock) Complete(ctx context.Context, req Request) (*Response, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(m.Latency):
	}

	if strings.Contains(req.System, "planner") {
		return &Response{Content: plannerStub(lastUser(req.Messages))}, nil
	}
	return &Response{Content: executorStub(lastUser(req.Messages))}, nil
}

func lastUser(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == RoleUser {
			return messages[i].Content
		}
	}
	return ""
}

func plannerStub(goal string) string {
	return fmt.Sprintf(`{
  "tasks": [
    {"title": "Entender o objetivo", "instruction": "Resuma em uma frase o que precisa ser feito: %s"},
    {"title": "Levantar premissas", "instruction": "Liste as premissas e restrições relevantes."},
    {"title": "Propor passos", "instruction": "Liste 3 a 5 passos concretos para atingir o objetivo."},
    {"title": "Consolidar resposta", "instruction": "Sintetize os passos anteriores em uma resposta final para o usuário."}
  ]
}`, strings.ReplaceAll(goal, `"`, `'`))
}

func executorStub(instruction string) string {
	return "[mock] " + strings.TrimSpace(instruction)
}
