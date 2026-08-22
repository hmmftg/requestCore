package renderers

import (
	"fmt"
	"io"
)

// TextRenderer encodes data as plain text.
//
// Accepted input types:
//   - string: written as-is
//   - []byte: written as-is
//   - fmt.Stringer: String() is called
//   - any other type: formatted with fmt.Sprint
type TextRenderer struct{}

// ContentType returns the plain text content type.
func (r TextRenderer) ContentType() string {
	return "text/plain; charset=utf-8"
}

// Encode serializes data as plain text.
func (r TextRenderer) Encode(data any) ([]byte, error) {
	switch v := data.(type) {
	case string:
		return []byte(v), nil
	case []byte:
		return v, nil
	case io.Reader:
		return io.ReadAll(v)
	case fmt.Stringer:
		return []byte(v.String()), nil
	default:
		return []byte(fmt.Sprint(data)), nil
	}
}
