package domain

// Turn is one full request/response cycle in a session: what the user
// typed, the goal the orchestrator actually planned for (which may include
// prior context for a refinement), the plan produced, and the final result.
type Turn struct {
	UserInput string
	Goal      string
	Plan      *Plan
	Result    string
}
