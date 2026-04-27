package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// defaultReadLimit caps the bytes returned to the LLM, both to keep
// prompts short and to prevent accidental exfiltration of huge files.
const defaultReadLimit = 256 * 1024

// ReadFile reads a UTF-8 text file from the sandbox.
type ReadFile struct {
	Sandbox  *Sandbox
	MaxBytes int // <=0 → defaultReadLimit
}

func (r *ReadFile) Name() string { return "read_file" }

func (r *ReadFile) Description() string {
	return "Read a UTF-8 text file from the workspace and return its contents."
}

func (r *ReadFile) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the file, relative to the workspace root.",
			},
		},
		"required":             []string{"path"},
		"additionalProperties": false,
	}
}

func (r *ReadFile) Run(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", errors.New("missing path")
	}
	abs, err := r.Sandbox.Resolve(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("path %q is a directory; use list_dir", path)
	}
	limit := int64(r.MaxBytes)
	if limit <= 0 {
		limit = defaultReadLimit
	}
	if info.Size() > limit {
		return "", fmt.Errorf("file %q is %d bytes (limit %d)", path, info.Size(), limit)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
