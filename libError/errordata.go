package libError

import (
	"log/slog"
	"strings"
	"time"
)

type ErrorData struct {
	Time       time.Time
	ActionData Action  `json:"action"`
	Source     *Source `json:"source,omitempty"`
	Child      Error   `json:"children"`
}

func (e ErrorData) Action() Action { return e.ActionData }
func (e ErrorData) Src() *Source   { return e.Source }
func (e ErrorData) Format(stack *strings.Builder) {
	e.ActionData.Format(stack)
	if e.Source != nil {
		e.Source.Format(stack)
	}
	if e.Child != nil {
		stack.WriteString("\n")
		e.Child.Format(stack)
	}
}

func (e ErrorData) SLog() slog.Attr {
	attrs := []any{
		slog.Time("time", e.Time),
		e.ActionData.SLog(),
	}
	if e.Source != nil {
		attrs = append(attrs, e.Source.SLog())
	}
	if e.Child != nil {
		attrs = append(attrs, slog.Group("child", e.Child.SLog()))
	}
	return slog.Group("error", attrs...)
}
