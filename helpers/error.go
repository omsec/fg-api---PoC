package helpers

import "fmt"

// SystemError wraps external errors (such as DB) and lets the caller add
// additional context information
type SystemError struct {
	Context string // eg. Function Name
	Err     error
}

func (se *SystemError) Error() string {
	return fmt.Sprintf("%s: %v", se.Context, se.Err)
}

// WrapError lets the caller add context information to another error
// (eg. after receiving a DB error)
func WrapError(err error, info string) *SystemError {
	return &SystemError{
		Context: info,
		Err:     err,
	}
}
