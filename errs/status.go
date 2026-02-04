package errs

// Status represents an error with an associated HTTP status code.
type Status struct {
	status  int    // HTTP status code
	message string // Error message
}

// Error returns the status error message.
func (e *Status) Error() string {
	return e.message
}

// Status returns the HTTP status code associated with this error.
func (e *Status) Status() int {
	return e.status
}

// NewStatusError creates a new status error with the given message and HTTP status code.
func NewStatusError(message string, status int) *Status {
	return &Status{
		message: message,
		status:  status,
	}
}
