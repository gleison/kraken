// Command kraken runs the TUI orchestrator.
//
// Usage:
//
//	ANTHROPIC_API_KEY=... kraken
//
// If ANTHROPIC_API_KEY is unset, a mock LLM is used so the UI can be
// explored without making network calls.
package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gleison/kraken/internal/llm"
	"github.com/gleison/kraken/internal/orchestrator"
	"github.com/gleison/kraken/internal/tui"
)

func main() {
	client := buildClient()
	orch := orchestrator.New(client)

	program := tea.NewProgram(tui.NewModel(orch), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		log.Fatalf("kraken: %v", err)
	}
}

func buildClient() llm.Client {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "kraken: ANTHROPIC_API_KEY not set, using mock LLM")
		return llm.NewMock()
	}
	model := os.Getenv("KRAKEN_MODEL")
	return llm.NewAnthropic(apiKey, model)
}
