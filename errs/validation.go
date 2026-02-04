package errs

// Validation represents a validation error with a descriptive message.
type Validation struct {
	message string // Error message describing the validation failure
}

// Error returns the validation error message.
func (e *Validation) Error() string {
	return e.message
}

// NewValidationError creates a new validation error with the given message.
func NewValidationError(message string) *Validation {
	return &Validation{message}
}
