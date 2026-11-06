package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const defaultWriteLimit = 1024 * 1024

type WriteFile struct {
	Sandbox  *Sandbox
	Confirm  func(path, content string) bool
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
	path, pathOK := args["path"].(string)
	if !pathOK || path == "" {
		return "", errors.New("missing path")
	}
	content, contentOK := args["content"].(string)
	if !contentOK {
		return "", errors.New("content must be a string")
	}
	abs, err := w.Sandbox.Resolve(path)
	if err != nil {
		return "", err
	}
	limit := w.MaxBytes
	if limit <= 0 {
		limit = defaultWriteLimit
	}
	if len(content) > limit {
		return "", fmt.Errorf("write of %d bytes exceeds limit (%d)", len(content), limit)
	}
	if w.Confirm != nil && !w.Confirm(abs, content) {
		return "", errors.New("write rejected")
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", fmt.Errorf("mkdir parent: %w", err)
	}
	if err := writeAtomically(abs, []byte(content)); err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(content), path), nil
}

func writeAtomically(path string, content []byte) (err error) {
	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		if info.IsDir() {
			return fmt.Errorf("path %q is a directory", path)
		}
		mode = info.Mode().Perm()
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".kraken-write-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		if err != nil {
			_ = os.Remove(tmpName)
		}
	}()

	if err = tmp.Chmod(mode); err != nil {
		return err
	}
	if _, err = tmp.Write(content); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmpName, path); err != nil {
		return err
	}
	return nil
}
