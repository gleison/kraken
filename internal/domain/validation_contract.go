package domain

type Validatable interface {
	Validate() error
}
