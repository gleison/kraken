package tools

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Sandbox confines tool I/O to a single root directory. Every path the
// LLM provides (relative or absolute) is resolved against the root, with
// symlinks evaluated, and any result that escapes the root is rejected.
type Sandbox struct {
	root string // absolute, symlinks resolved
}

// NewSandbox builds a Sandbox rooted at root. The directory must exist.
func NewSandbox(root string) (*Sandbox, error) {
	if root == "" {
		return nil, errors.New("sandbox: empty root")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("sandbox abs: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("sandbox resolve %q: %w", root, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("sandbox stat %q: %w", resolved, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("sandbox root %q is not a directory", resolved)
	}
	return &Sandbox{root: resolved}, nil
}

// Root returns the resolved root directory.
func (s *Sandbox) Root() string { return s.root }

// Resolve maps an LLM-supplied path to an absolute path inside the root.
// Returns an error for empty paths, paths that escape the root, and paths
// that traverse symlinks pointing outside the root.
func (s *Sandbox) Resolve(p string) (string, error) {
	if p == "" {
		return "", errors.New("path is empty")
	}
	var abs string
	if filepath.IsAbs(p) {
		abs = filepath.Clean(p)
	} else {
		abs = filepath.Clean(filepath.Join(s.root, p))
	}
	// Resolve symlinks for the deepest existing prefix; the leaf may not
	// exist yet (e.g. when writing a new file).
	resolved := s.resolveExisting(abs)
	if !s.contains(resolved) {
		return "", fmt.Errorf("path %q escapes the workspace", p)
	}
	return abs, nil
}

// resolveExisting walks the path inward, returning the result of
// EvalSymlinks for the longest prefix that exists, with the remaining
// (non-existent) tail re-attached. This lets write_file create new files
// without symlink errors.
func (s *Sandbox) resolveExisting(abs string) string {
	if r, err := filepath.EvalSymlinks(abs); err == nil {
		return r
	}
	parent := filepath.Dir(abs)
	if parent == abs {
		return abs
	}
	parentResolved := s.resolveExisting(parent)
	return filepath.Join(parentResolved, filepath.Base(abs))
}

func (s *Sandbox) contains(abs string) bool {
	if abs == s.root {
		return true
	}
	prefix := s.root + string(filepath.Separator)
	return strings.HasPrefix(abs, prefix)
}
