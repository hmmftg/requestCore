package libError

import "runtime"

func getStack() *Source {
	_, filename, line, _ := runtime.Caller(2)
	return &Source{filename, line}
}

// GetStack returns the Source of the immediate caller.
func GetStack() *Source {
	_, filename, line, _ := runtime.Caller(1)
	return &Source{filename, line}
}
