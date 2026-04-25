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
//	OPENAI_TIMEOUT   Per-request timeout in seconds, default 600 (10 min)
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

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
		Timeout: parseTimeoutSeconds(os.Getenv("OPENAI_TIMEOUT")),
	})
}

// parseTimeoutSeconds reads a non-negative integer (seconds) and returns it
// as a Duration. Empty or invalid values fall through to the library default.
func parseTimeoutSeconds(s string) time.Duration {
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		fmt.Fprintf(os.Stderr, "kraken: ignoring invalid OPENAI_TIMEOUT=%q\n", s)
		return 0
	}
	return time.Duration(n) * time.Second
}
