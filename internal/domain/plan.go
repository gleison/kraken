package domain

// Plan is a decomposition of a complex Goal into a sequence of simple Tasks.
type Plan struct {
	Goal  string
	Tasks []*Task
}

// NewPlan builds a Plan for the given goal.
func NewPlan(goal string, tasks []*Task) *Plan {
	return &Plan{Goal: goal, Tasks: tasks}
}

// IsEmpty reports whether the plan has no tasks.
func (p *Plan) IsEmpty() bool {
	return p == nil || len(p.Tasks) == 0
}
