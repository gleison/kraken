package domain

type Turn struct {
	UserInput string
	Goal      string
	Plan      *Plan
	Result    string
}
