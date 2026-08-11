package libError

import "errors"

// Unwrap attempts to extract a structured Error from a standard error, returning whether it succeeded.
func Unwrap(err error) (bool, Error) {
	errData := &ErrorData{}
	if errors.As(err, errData) {
		return true, errData
	}
	return false, nil
}
