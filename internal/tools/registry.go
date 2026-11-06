package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
)

var _ Catalog = (*Registry)(nil)

type Registry struct {
	tools map[string]Tool
}

func NewRegistry(tools ...Tool) *Registry {
	m := make(map[string]Tool, len(tools))
	for _, t := range tools {
		m[t.Name()] = t
	}
	return &Registry{tools: m}
}

func (r *Registry) List() []Tool {
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name() < out[j].Name()
	})
	return out
}

func (r *Registry) Has(name string) bool {
	_, ok := r.tools[name]
	return ok
}

func (r *Registry) Empty() bool {
	return len(r.tools) == 0
}

// Invoke decodes the call's raw arguments (if needed) and executes the
// matching tool, returning the populated Result.
func (r *Registry) Invoke(ctx context.Context, c Call) Result {
	res := Result{Call: c}
	t, ok := r.tools[c.Name]
	if !ok {
		res.Err = fmt.Errorf("unknown tool %q", c.Name)
		return res
	}
	args := c.Args
	if args == nil && c.Raw != "" {
		args = map[string]any{}
		if err := json.Unmarshal([]byte(c.Raw), &args); err != nil {
			res.Err = fmt.Errorf("decode args: %w", err)
			return res
		}
	}
	out, err := t.Run(ctx, args)
	res.Content = out
	res.Err = err
	return res
}
