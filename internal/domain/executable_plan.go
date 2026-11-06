package domain

type ExecutablePlan struct {
	ID         string     `json:"id"`
	Goal       string     `json:"goal"`
	Activities []Activity `json:"activities"`
	Budget     Budget     `json:"budget"`
}
