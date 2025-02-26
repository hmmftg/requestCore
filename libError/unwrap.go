package libError

import "errors"

func Unwrap(err error) (bool, Error) {
	errData := &ErrorData{}
	if errors.As(err, errData) {
		return true, errData
	}
	return false, nil
}
