package domain

type Plan struct {
	Goal  string
	Tasks []*Task
}

func NewPlan(goal string, tasks []*Task) *Plan {
	return &Plan{Goal: goal, Tasks: tasks}
}

func (p *Plan) Clone() *Plan {
	if p == nil {
		return nil
	}
	tasks := make([]*Task, len(p.Tasks))
	for i, task := range p.Tasks {
		tasks[i] = task.Clone()
	}
	return &Plan{Goal: p.Goal, Tasks: tasks}
}

func (p *Plan) IsEmpty() bool {
	return p == nil || len(p.Tasks) == 0
}
