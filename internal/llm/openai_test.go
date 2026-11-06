package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestOpenAICompleteSendsToolsAndParsesToolCalls(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("authorization = %q", got)
		}
		var payload oaiRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Tools) != 1 || payload.Tools[0].Function.Name != "read_file" {
			t.Fatalf("unexpected tools: %#v", payload.Tools)
		}
		return jsonResponse(http.StatusOK, `{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call-1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"README.md\"}"}}]}}]}`), nil
	})

	client := NewOpenAI(Config{BaseURL: "http://llm.test", APIKey: "secret", Model: "test"})
	client.http.Transport = transport
	response, err := client.Complete(context.Background(), Request{
		Messages: []Message{{Role: RoleUser, Content: "inspect"}},
		Tools:    []Tool{{Name: "read_file", Schema: map[string]any{"type": "object"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].Args != `{"path":"README.md"}` {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestOpenAICachesResponseFormatFallback(t *testing.T) {
	var mu sync.Mutex
	var formats []string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var payload oaiRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		format := "none"
		if payload.ResponseFormat != nil {
			format = payload.ResponseFormat.Type
		}
		mu.Lock()
		formats = append(formats, format)
		mu.Unlock()
		if format == "json_schema" {
			return jsonResponse(http.StatusBadRequest, `{"error":{"message":"response_format json_schema is unsupported"}}`), nil
		}
		return jsonResponse(http.StatusOK, `{"choices":[{"message":{"role":"assistant","content":"{}"}}]}`), nil
	})

	client := NewOpenAI(Config{BaseURL: "http://llm.test", Model: "test"})
	client.http.Transport = transport
	request := Request{JSONSchema: &JSONSchema{Name: "result", Schema: map[string]any{"type": "object"}}}
	for range 2 {
		if _, err := client.Complete(context.Background(), request); err != nil {
			t.Fatal(err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{"json_schema", "json_object", "json_object"}
	if len(formats) != len(want) {
		t.Fatalf("formats = %v", formats)
	}
	for i := range want {
		if formats[i] != want[i] {
			t.Fatalf("formats = %v, want %v", formats, want)
		}
	}
}
