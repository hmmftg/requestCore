package libError

import (
	"log/slog"
	"strings"
)

type Error interface {
	error
	Action() Action
	Src() *Source
	Format(*strings.Builder)
	SLog() slog.Attr
}
