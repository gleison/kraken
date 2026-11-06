package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSandboxResolve(t *testing.T) {
	root := t.TempDir()
	sandbox, err := NewSandbox(root)
	if err != nil {
		t.Fatal(err)
	}

	inside, err := sandbox.Resolve("dir/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if inside != filepath.Join(sandbox.Root(), "dir", "file.txt") {
		t.Fatalf("unexpected path %q", inside)
	}
	if _, err := sandbox.Resolve("../outside.txt"); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
	if _, err := sandbox.Resolve(filepath.Join(root, "..", "outside.txt")); err == nil {
		t.Fatal("expected absolute escape to be rejected")
	}
}

func TestSandboxRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated privileges")
	}
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	sandbox, err := NewSandbox(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sandbox.Resolve("escape/secret.txt"); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}
