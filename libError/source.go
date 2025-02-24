package libError

import (
	"fmt"
	"log/slog"
	"strings"
)

type Source struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

func (s Source) Format(stack *strings.Builder) {
	stack.WriteString(fmt.Sprintf("%s:%d", s.File, s.Line))
}

// LogValue implements slog.LogValuer and returns a grouped value
// with fields redacted. See https://pkg.go.dev/log/slog#LogValuer
func (s Source) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("file", s.File),
		slog.Int("line", s.Line),
	)
}
