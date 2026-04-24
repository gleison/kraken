// Command kraken runs the TUI orchestrator.
//
// The orchestrator talks to any endpoint that implements the OpenAI
// Chat Completions protocol (OpenAI, Azure OpenAI, Groq, Together,
// OpenRouter, Ollama, vLLM, LM Studio, ...).
//
// Configuration via environment variables:
//
//	OPENAI_API_KEY   API key (required for real runs; if empty, uses mock)
//	OPENAI_BASE_URL  Base URL, default https://api.openai.com/v1
//	OPENAI_MODEL     Model name, default gpt-4o-mini
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
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "kraken: OPENAI_API_KEY not set, using mock LLM")
		return llm.NewMock()
	}
	return llm.NewOpenAI(llm.Config{
		APIKey:  apiKey,
		BaseURL: os.Getenv("OPENAI_BASE_URL"),
		Model:   os.Getenv("OPENAI_MODEL"),
	})
}
