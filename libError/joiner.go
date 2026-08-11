package libError

import "fmt"

// Join wraps err with a formatted message, preserving the original error for unwrapping.
func Join(err error, format string, args ...any) error {
	return fmt.Errorf("%w; %s", err, fmt.Sprintf(format, args...))
}

// Append joins errChild to err with a formatted message, handling nil err gracefully.
func Append(err error, errChild error, format string, args ...any) error {
	if err == nil {
		return Join(errChild, format, args...)
	}
	return fmt.Errorf("%w; %w; %s", err, errChild, fmt.Sprintf(format, args...))
}
