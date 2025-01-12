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

func (s Source) SLog() slog.Attr {
	attrs := []any{
		slog.String("file", s.File),
		slog.Int("line", s.Line),
	}
	return slog.Group("source", attrs...)
}
