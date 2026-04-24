package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	anthropicEndpoint = "https://api.anthropic.com/v1/messages"
	anthropicVersion  = "2023-06-01"
	defaultModel      = "claude-opus-4-7"
	defaultMaxTokens  = 1024
	defaultTimeout    = 60 * time.Second
)

// Anthropic is a minimal HTTP client for the Messages API.
// Keeping it provider-specific but thin avoids pulling a full SDK.
type Anthropic struct {
	apiKey string
	model  string
	http   *http.Client
}

// NewAnthropic builds a client with sane defaults.
func NewAnthropic(apiKey, model string) *Anthropic {
	if model == "" {
		model = defaultModel
	}
	return &Anthropic{
		apiKey: apiKey,
		model:  model,
		http:   &http.Client{Timeout: defaultTimeout},
	}
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete implements llm.Client.
func (a *Anthropic) Complete(ctx context.Context, req Request) (*Response, error) {
	if a.apiKey == "" {
		return nil, errors.New("anthropic: missing API key")
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}

	msgs := make([]anthropicMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, anthropicMessage{Role: string(m.Role), Content: m.Content})
	}

	body, err := json.Marshal(anthropicRequest{
		Model:     a.model,
		MaxTokens: maxTokens,
		System:    req.System,
		Messages:  msgs,
	})
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := a.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: http: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: read body: %w", err)
	}

	var parsed anthropicResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("anthropic: decode response: %w (body=%s)", err, string(raw))
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("anthropic: %s: %s", parsed.Error.Type, parsed.Error.Message)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("anthropic: http %d: %s", resp.StatusCode, string(raw))
	}

	var text string
	for _, block := range parsed.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}
	return &Response{Content: text}, nil
}
