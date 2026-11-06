package orchestrator

import "github.com/gleison/kraken/internal/llm"

// New supplies the default LLM-backed planner and executor. Drivers that need
// tool configuration or alternate strategies compose them with NewWithPorts.
func New(client llm.Client) *Orchestrator {
	return NewWithPorts(NewPlanner(client), NewExecutor(client))
}
