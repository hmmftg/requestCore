package libError

import (
	"log/slog"
	"strings"
	"time"
)

// ErrorData is the concrete implementation of Error, carrying time, action, source, and child error.
type ErrorData struct {
	Time       time.Time
	ActionData Action  `json:"action"`
	Source     *Source `json:"source,omitempty"`
	Child      Error   `json:"child"`
}

// Action returns the action data associated with this error.
func (e ErrorData) Action() Action { return e.ActionData }

// Src returns the source location where this error was created.
func (e ErrorData) Src() *Source { return e.Source }

// Format writes the full error chain (action, source, and child) into the given string builder.
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

// LogValue implements slog.LogValuer and returns a grouped value
// with fields redacted. See https://pkg.go.dev/log/slog#LogValuer
func (e ErrorData) LogValue() slog.Value {
	var src slog.Attr
	if e.Source != nil {
		src = slog.Any("source", e.Source)
	} else {
		src = slog.Attr{}
	}
	return slog.GroupValue(
		slog.Time("time", e.Time),
		slog.String("desc", e.ActionData.Description),
		slog.Any("action", e.ActionData),
		src,
		slog.Any("child", e.Child),
	)
}
