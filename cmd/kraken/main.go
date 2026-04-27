// Command kraken runs the TUI orchestrator.
//
// The orchestrator talks to any endpoint that implements the OpenAI
// Chat Completions protocol (OpenAI, Azure OpenAI, Groq, Together,
// OpenRouter, Ollama, vLLM, LM Studio, ...).
//
// Configuration via environment variables. At least one of
// OPENAI_API_KEY or OPENAI_BASE_URL must be set, otherwise kraken
// exits with an error - there is no mock fallback.
//
//	OPENAI_API_KEY     API key (omit for keyless local providers
//	                   like LM Studio / Ollama).
//	OPENAI_BASE_URL    Base URL, default https://api.openai.com/v1.
//	OPENAI_MODEL       Model name, default gpt-4o-mini.
//	OPENAI_TIMEOUT     Per-request timeout in seconds, default 600 (10 min).
//	OPENAI_MAX_TOKENS  Max tokens per response, default 4096.
//	KRAKEN_LOG         Path to a debug log file. When set, every HTTP
//	                   request and planner stage is recorded with a
//	                   timestamp; useful when the TUI seems stuck.
package main

import (
	"fmt"
	"io"
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
	closeLog := setupLog()
	defer closeLog()

	client, err := buildClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kraken:", err)
		os.Exit(2)
	}
	orch := orchestrator.New(client)

	program := tea.NewProgram(tui.NewModel(orch), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		log.Fatalf("kraken: %v", err)
	}
}

// setupLog wires the standard logger to the file pointed to by KRAKEN_LOG,
// or silences it otherwise (so log output does not corrupt the TUI). The
// returned closer flushes and closes the file.
func setupLog() func() {
	path := os.Getenv("KRAKEN_LOG")
	if path == "" {
		log.SetOutput(io.Discard)
		return func() {}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		// Fall back silently; logging is best-effort.
		log.SetOutput(io.Discard)
		return func() {}
	}
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("kraken: log opened (pid=%d)", os.Getpid())
	return func() {
		log.Printf("kraken: log closed")
		_ = f.Close()
	}
}

func buildClient() (llm.Client, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	model := os.Getenv("OPENAI_MODEL")

	if apiKey == "" && baseURL == "" {
		return nil, fmt.Errorf(
			"no LLM configured: set OPENAI_API_KEY (cloud) or OPENAI_BASE_URL (e.g. http://localhost:1234/v1 for LM Studio)")
	}
	log.Printf("kraken: using OpenAI client (base=%q model=%q has_key=%t)",
		baseURL, model, apiKey != "")
	return llm.NewOpenAI(llm.Config{
		APIKey:    apiKey,
		BaseURL:   baseURL,
		Model:     model,
		Timeout:   parseTimeoutSeconds(os.Getenv("OPENAI_TIMEOUT")),
		MaxTokens: parsePositiveInt("OPENAI_MAX_TOKENS", os.Getenv("OPENAI_MAX_TOKENS")),
	}), nil
}

// parseTimeoutSeconds reads a non-negative integer (seconds) and returns it
// as a Duration. Empty or invalid values fall through to the library default.
func parseTimeoutSeconds(s string) time.Duration {
	n := parsePositiveInt("OPENAI_TIMEOUT", s)
	return time.Duration(n) * time.Second
}

// parsePositiveInt returns the non-negative integer encoded in s, or 0 if
// s is empty or invalid. The name is used only for error reporting.
func parsePositiveInt(name, s string) int {
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		fmt.Fprintf(os.Stderr, "kraken: ignoring invalid %s=%q\n", name, s)
		return 0
	}
	return n
}
