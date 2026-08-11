package libError

import (
	"fmt"
	"log/slog"
	"strings"
)

// Source identifies the file and line where an error originated.
type Source struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

// Format writes the source file and line number into the given string builder.
func (s Source) Format(stack *strings.Builder) {
	fmt.Fprintf(stack, "%s:%d", s.File, s.Line)
}

// LogValue implements slog.LogValuer and returns a grouped value
// with fields redacted. See https://pkg.go.dev/log/slog#LogValuer
func (s Source) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("file", s.File),
		slog.Int("line", s.Line),
	)
}
