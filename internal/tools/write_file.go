package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// WriteFile creates or replaces a UTF-8 text file in the sandbox.
//
// It is the only tool with side effects, so it is gated behind an
// explicit flag at construction (the registry simply does not include
// it when writes are disallowed) and an optional Confirm callback that
// can prompt the user for each call.
type WriteFile struct {
	Sandbox *Sandbox

	// Confirm, if non-nil, is consulted before each write. Returning
	// false aborts the call with an error and the file is not touched.
	Confirm func(path, content string) bool

	// MaxBytes caps the size of a single write. <=0 disables the check.
	MaxBytes int
}

func (w *WriteFile) Name() string { return "write_file" }

func (w *WriteFile) Description() string {
	return "Create or replace a UTF-8 text file in the workspace. Use this only after you are confident in the final content; it overwrites the existing file."
}

func (w *WriteFile) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the file, relative to the workspace root.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Full file contents to write.",
			},
		},
		"required":             []string{"path", "content"},
		"additionalProperties": false,
	}
}

func (w *WriteFile) Run(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)
	if path == "" {
		return "", errors.New("missing path")
	}
	abs, err := w.Sandbox.Resolve(path)
	if err != nil {
		return "", err
	}
	if w.MaxBytes > 0 && len(content) > w.MaxBytes {
		return "", fmt.Errorf("write of %d bytes exceeds limit (%d)", len(content), w.MaxBytes)
	}
	if w.Confirm != nil && !w.Confirm(abs, content) {
		return "", errors.New("write rejected")
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", fmt.Errorf("mkdir parent: %w", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(content), path), nil
}
