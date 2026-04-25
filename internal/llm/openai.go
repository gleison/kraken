package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

const (
	defaultBaseURL   = "https://api.openai.com/v1"
	defaultModel     = "gpt-4o-mini"
	defaultMaxTokens = 4096
	// defaultTimeout is generous on purpose: local models (e.g. Gemma 2B
	// behind llama.cpp/Ollama) often need several minutes to emit long
	// code outputs. Tighten via Config.Timeout / OPENAI_TIMEOUT when
	// using a fast hosted endpoint.
	defaultTimeout = 10 * time.Minute
)

// formatLevel ranks response_format variants from strictest (0) to none (2).
// The adapter remembers the first level accepted by the endpoint, so the
// fallback is paid at most once per client.
type formatLevel int32

const (
	levelSchema formatLevel = iota // response_format: {type: "json_schema", ...}
	levelObject                    // response_format: {type: "json_object"}
	levelNone                      // no response_format
)

// OpenAI is a thin adapter for any endpoint that speaks the
// OpenAI Chat Completions protocol (OpenAI, Azure, Groq, Together,
// OpenRouter, Ollama, vLLM, LM Studio, ...).
type OpenAI struct {
	apiKey    string
	baseURL   string
	model     string
	maxTokens int
	http      *http.Client

	// minLevel is the lowest-strictness response_format this endpoint
	// is known to accept. Starts at levelSchema and only ever increases.
	minLevel atomic.Int32
}

// Config groups the OpenAI adapter parameters.
// Zero-value fields fall back to library defaults.
type Config struct {
	APIKey    string
	BaseURL   string
	Model     string
	Timeout   time.Duration
	MaxTokens int
}

// NewOpenAI builds the client. Only APIKey is strictly required.
func NewOpenAI(cfg Config) *OpenAI {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	model := cfg.Model
	if model == "" {
		model = defaultModel
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &OpenAI{
		apiKey:    cfg.APIKey,
		baseURL:   strings.TrimRight(baseURL, "/"),
		model:     model,
		maxTokens: maxTokens,
		http:      &http.Client{Timeout: timeout},
	}
}

type oaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type oaiJSONSchema struct {
	Name   string `json:"name"`
	Strict bool   `json:"strict,omitempty"`
	Schema any    `json:"schema"`
}

type oaiResponseFormat struct {
	Type       string         `json:"type"`
	JSONSchema *oaiJSONSchema `json:"json_schema,omitempty"`
}

type oaiRequest struct {
	Model          string             `json:"model"`
	Messages       []oaiMessage       `json:"messages"`
	MaxTokens      int                `json:"max_tokens,omitempty"`
	ResponseFormat *oaiResponseFormat `json:"response_format,omitempty"`
}

type oaiResponse struct {
	Choices []struct {
		Message oaiMessage `json:"message"`
	} `json:"choices"`
	Error json.RawMessage `json:"error,omitempty"`
}

// parseError extracts a human-readable message from the error field,
// which different providers shape differently: an object with "message",
// an object with "code"/"message", or a bare string.
func parseError(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var obj struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		switch {
		case obj.Message != "":
			return obj.Message
		case obj.Code != "":
			return obj.Code
		}
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s != "" {
		return s
	}
	return string(raw)
}

// Complete implements llm.Client. When req.JSONSchema is set, it tries
// response_format variants in decreasing strictness (json_schema → json_object
// → none) until the endpoint accepts one, then remembers that level for
// subsequent calls on this client.
func (o *OpenAI) Complete(ctx context.Context, req Request) (*Response, error) {
	if o.apiKey == "" {
		return nil, errors.New("openai: missing API key")
	}

	if req.JSONSchema == nil {
		return o.doRequest(ctx, req, levelNone)
	}

	level := formatLevel(o.minLevel.Load())
	for ; level <= levelNone; level++ {
		resp, err := o.doRequest(ctx, req, level)
		if err == nil {
			o.minLevel.Store(int32(level))
			return resp, nil
		}
		if !isFormatRejection(err) {
			return nil, err
		}
	}
	return nil, errors.New("openai: provider rejected every response_format variant")
}

// doRequest sends a single request using the given response_format level.
func (o *OpenAI) doRequest(ctx context.Context, req Request, level formatLevel) (*Response, error) {
	messages := make([]oaiMessage, 0, len(req.Messages)+1)
	if req.System != "" {
		messages = append(messages, oaiMessage{Role: "system", Content: req.System})
	}
	for _, m := range req.Messages {
		messages = append(messages, oaiMessage{Role: string(m.Role), Content: m.Content})
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = o.maxTokens
	}

	payload := oaiRequest{
		Model:          o.model,
		Messages:       messages,
		MaxTokens:      maxTokens,
		ResponseFormat: buildResponseFormat(req.JSONSchema, level),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal: %w", err)
	}

	url := o.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: http: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai: read body: %w", err)
	}

	var parsed oaiResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("openai: decode: %w (status=%d body=%s)", err, resp.StatusCode, truncate(string(raw), 500))
	}
	if msg := parseError(parsed.Error); msg != "" {
		return nil, fmt.Errorf("openai: %s", msg)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("openai: http %d: %s", resp.StatusCode, truncate(string(raw), 500))
	}
	if len(parsed.Choices) == 0 {
		return nil, errors.New("openai: empty response")
	}

	return &Response{Content: parsed.Choices[0].Message.Content}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// buildResponseFormat returns the response_format payload for the given
// strictness level, or nil when no constraint should be sent.
func buildResponseFormat(schema *JSONSchema, level formatLevel) *oaiResponseFormat {
	if schema == nil {
		return nil
	}
	switch level {
	case levelSchema:
		return &oaiResponseFormat{
			Type: "json_schema",
			JSONSchema: &oaiJSONSchema{
				Name:   schema.Name,
				Strict: schema.Strict,
				Schema: schema.Schema,
			},
		}
	case levelObject:
		return &oaiResponseFormat{Type: "json_object"}
	default:
		return nil
	}
}

// isFormatRejection reports whether err looks like the provider refusing the
// response_format variant we sent. Detection is string-based because
// providers report this in heterogeneous ways.
func isFormatRejection(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "response_format") ||
		strings.Contains(msg, "json_schema") ||
		strings.Contains(msg, "json_object")
}
