package libError

import "fmt"

func Join(err error, format string, args ...any) error {
	return fmt.Errorf("%w; %s", err, fmt.Sprintf(format, args...))
}

func Append(err error, errChild error, format string, args ...any) error {
	if err == nil {
		return Join(errChild, format, args...)
	}
	return fmt.Errorf("%w; %w; %s", err, errChild, fmt.Sprintf(format, args...))
}
