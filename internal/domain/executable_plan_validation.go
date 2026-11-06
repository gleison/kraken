package domain

import "fmt"

func (plan *ExecutablePlan) Validate() error {
	if plan == nil {
		return fmt.Errorf("plan: nil contract")
	}
	if err := validateIdentifier("plan.id", plan.ID); err != nil {
		return err
	}
	if err := validateRequiredText("plan.goal", plan.Goal); err != nil {
		return err
	}
	if err := plan.Budget.Validate(); err != nil {
		return fmt.Errorf("plan: %w", err)
	}
	if len(plan.Activities) == 0 {
		return fmt.Errorf("plan.activities: at least one activity is required")
	}
	for index := range plan.Activities {
		if err := plan.Activities[index].Validate(); err != nil {
			return fmt.Errorf("plan.activities[%d]: %w", index, err)
		}
	}
	return nil
}
