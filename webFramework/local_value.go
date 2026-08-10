package webFramework

// GetLocalOrDefault retrieves a parser-local value by key.
// It returns defaultValue when w.Parser is nil, the local is absent, or the
// stored value is not of type T.
//
// GetLocalOrDefault is read-only: it never mutates parser locals and never
// registers package-level state. It is safe for concurrent use as long as the
// underlying RequestParser implementation is safe for concurrent reads.
func GetLocalOrDefault[T any](w WebFramework, key string, defaultValue T) T {
	if w.Parser == nil {
		return defaultValue
	}
	value := w.Parser.GetLocal(key)
	if value == nil {
		return defaultValue
	}
	typed, ok := value.(T)
	if !ok {
		return defaultValue
	}
	return typed
}
