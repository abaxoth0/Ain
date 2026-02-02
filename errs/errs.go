package errs

type Status struct {
	status  int
	message string
}

func (e *Status) Error() string {
	return e.message
}

func (e *Status) Status() int {
	return e.status
}

func NewStatusError(message string, status int) *Status {
	return &Status{
		message: message,
		status:  status,
	}
}
