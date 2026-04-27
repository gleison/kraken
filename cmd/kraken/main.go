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
//	OPENAI_API_KEY      API key (omit for keyless local providers
//	                    like LM Studio / Ollama).
//	OPENAI_BASE_URL     Base URL, default https://api.openai.com/v1.
//	OPENAI_MODEL        Model name, default gpt-4o-mini.
//	OPENAI_TIMEOUT      Per-request timeout in seconds, default 600 (10 min).
//	OPENAI_MAX_TOKENS   Max tokens per response, default 4096.
//	KRAKEN_LOG          Path to a debug log file. When set, every HTTP
//	                    request and planner stage is recorded with a
//	                    timestamp; useful when the TUI seems stuck.
//	KRAKEN_WORKSPACE    Directory the file tools may read/write inside.
//	                    Default: the current working directory.
//	KRAKEN_ALLOW_WRITE  Set to "1" to enable the write_file tool. Off by
//	                    default; read_file and list_dir are always on.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gleison/kraken/internal/llm"
	"github.com/gleison/kraken/internal/orchestrator"
	"github.com/gleison/kraken/internal/tools"
	"github.com/gleison/kraken/internal/tui"
)

func main() {
	allowWriteFlag := flag.Bool("write", false, "enable the write_file tool (also: KRAKEN_ALLOW_WRITE=1)")
	workspaceFlag := flag.String("workspace", "", "directory the file tools may read/write inside (default: cwd, also: KRAKEN_WORKSPACE)")
	flag.Parse()

	closeLog := setupLog()
	defer closeLog()

	client, err := buildClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kraken:", err)
		os.Exit(2)
	}

	registry, err := buildToolRegistry(*workspaceFlag, *allowWriteFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kraken:", err)
		os.Exit(2)
	}

	orch := orchestrator.New(client)
	if registry != nil && !registry.Empty() {
		orch.Executor().WithTools(registry)
		orch.Planner().AnnounceTools(toolDescriptions(registry))
	}

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

// buildToolRegistry wires the workspace tools. read_file and list_dir are
// always on; write_file is gated behind --write or KRAKEN_ALLOW_WRITE=1.
func buildToolRegistry(workspaceFlag string, allowWriteFlag bool) (*tools.Registry, error) {
	root := workspaceFlag
	if root == "" {
		root = os.Getenv("KRAKEN_WORKSPACE")
	}
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("workspace: %w", err)
		}
	}
	sb, err := tools.NewSandbox(root)
	if err != nil {
		return nil, fmt.Errorf("workspace: %w", err)
	}

	allowWrite := allowWriteFlag || os.Getenv("KRAKEN_ALLOW_WRITE") == "1"
	log.Printf("kraken: workspace=%q allow_write=%t", sb.Root(), allowWrite)

	regList := []tools.Tool{
		&tools.ReadFile{Sandbox: sb},
		&tools.ListDir{Sandbox: sb},
	}
	if allowWrite {
		regList = append(regList, &tools.WriteFile{Sandbox: sb})
	}
	return tools.NewRegistry(regList...), nil
}

// toolDescriptions returns short "name — description" lines for each tool,
// suitable for the planner's tool-hint prompt.
func toolDescriptions(r *tools.Registry) []string {
	list := r.List()
	out := make([]string, 0, len(list))
	for _, t := range list {
		out = append(out, fmt.Sprintf("%s — %s", t.Name(), t.Description()))
	}
	return out
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
