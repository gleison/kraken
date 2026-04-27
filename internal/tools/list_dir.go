package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

// ListDir lists entries (files and subdirectories) under a sandbox path.
type ListDir struct {
	Sandbox *Sandbox
}

func (l *ListDir) Name() string { return "list_dir" }

func (l *ListDir) Description() string {
	return "List files and subdirectories in a workspace directory. Returns one entry per line: \"dir  name\" or \"file name (size)\"."
}

func (l *ListDir) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Directory path relative to the workspace root. Use \".\" for the root.",
			},
		},
		"required":             []string{"path"},
		"additionalProperties": false,
	}
}

func (l *ListDir) Run(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", errors.New("missing path")
	}
	abs, err := l.Sandbox.Resolve(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path %q is not a directory", path)
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return "", err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	var b strings.Builder
	for _, e := range entries {
		if e.IsDir() {
			fmt.Fprintf(&b, "dir   %s/\n", e.Name())
			continue
		}
		fi, err := e.Info()
		if err != nil {
			fmt.Fprintf(&b, "file  %s\n", e.Name())
			continue
		}
		fmt.Fprintf(&b, "file  %s (%d bytes)\n", e.Name(), fi.Size())
	}
	if b.Len() == 0 {
		return "(empty)\n", nil
	}
	return b.String(), nil
}
