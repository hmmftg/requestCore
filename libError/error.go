package libError

import (
	"log/slog"
	"strings"
)

// Error is the interface for structured errors with action, source, and logging support.
type Error interface {
	error
	Action() Action
	Src() *Source
	Format(*strings.Builder)
	LogValue() slog.Value
}
