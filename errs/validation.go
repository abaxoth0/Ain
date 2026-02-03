package errs

type Validation struct {
	message string
}

func (e *Validation) Error() string {
	return e.message
}

func NewValidationError(message string) *Validation {
	return &Validation{message}
}
