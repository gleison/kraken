package domain

func NewExecutablePlan(contract ExecutablePlan) (*ExecutablePlan, error) {
	if err := contract.Validate(); err != nil {
		return nil, err
	}
	return contract.Clone(), nil
}

func (plan *ExecutablePlan) Clone() *ExecutablePlan {
	if plan == nil {
		return nil
	}
	snapshot := *plan
	if plan.Activities != nil {
		snapshot.Activities = make([]Activity, len(plan.Activities))
		for index := range plan.Activities {
			snapshot.Activities[index] = *plan.Activities[index].Clone()
		}
	}
	return &snapshot
}
