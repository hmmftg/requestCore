package libError

import "fmt"

func Join(err error, format string, args ...any) error {
	return fmt.Errorf("%w; %s", err, fmt.Sprintf(format, args...))
}
