package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileRejectsMissingOrInvalidContent(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "important.txt")
	if err := os.WriteFile(path, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	sandbox, err := NewSandbox(root)
	if err != nil {
		t.Fatal(err)
	}
	tool := &WriteFile{Sandbox: sandbox}

	for name, args := range map[string]map[string]any{
		"missing": {"path": "important.txt"},
		"invalid": {"path": "important.txt", "content": 42},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := tool.Run(context.Background(), args); err == nil {
				t.Fatal("expected an error")
			}
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != "keep me" {
				t.Fatalf("file was modified: %q", content)
			}
		})
	}
}

func TestWriteFileReplacesAtomicallyAndPreservesMode(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "nested", "file.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	sandbox, err := NewSandbox(root)
	if err != nil {
		t.Fatal(err)
	}
	tool := &WriteFile{Sandbox: sandbox}

	if _, err := tool.Run(context.Background(), map[string]any{
		"path": "nested/file.txt", "content": "new",
	}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new" {
		t.Fatalf("got %q", content)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode changed to %o", info.Mode().Perm())
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".kraken-write-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files left behind: %v", matches)
	}
}

func TestWriteFileEnforcesDefaultLimit(t *testing.T) {
	root := t.TempDir()
	sandbox, err := NewSandbox(root)
	if err != nil {
		t.Fatal(err)
	}
	tool := &WriteFile{Sandbox: sandbox}
	_, err = tool.Run(context.Background(), map[string]any{
		"path": "large.txt", "content": strings.Repeat("x", defaultWriteLimit+1),
	})
	if err == nil {
		t.Fatal("expected size limit error")
	}
	if _, statErr := os.Stat(filepath.Join(root, "large.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("file should not exist, stat error: %v", statErr)
	}
}
