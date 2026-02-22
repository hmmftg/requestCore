package libError

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/hmmftg/requestCore/status"
)

// Action holds error action data. Description is the public code (e.g. API_OK_RESP_JSON).
// PublicDescription, when non-empty, is used as the client-visible description; otherwise
// the response layer resolves description from ErrorDesc/safe fallback. Message is internal-only (logs/tracing).
type Action struct {
	Status             status.StatusCode `json:"status"`
	Description        string            `json:"description"`         // public code
	PublicDescription  string            `json:"publicDescription"` // optional client-visible description
	Message            any               `json:"message"`            // internal only, never sent to client
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func ToSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToUpper(snake)
}

func (a Action) Format(stack *strings.Builder) {
	stack.WriteString(fmt.Sprintf(
		"status: %d|%s, desc: %s",
		a.Status, a.Status.String(),
		ToSnakeCase(a.Description),
	))
	message := ""
	if a.Message != nil {
		message = fmt.Sprintf(", message: {%+v}", a.Message)
		message = strings.ReplaceAll(message, "\n", "-")
	}
	stack.WriteString(message)
}

func (a Action) SLog() slog.Attr {
	attrs := []any{
		slog.String("status", fmt.Sprintf("%d|%s", a.Status.Int(), a.Status.String())),
		slog.String("desc", ToSnakeCase(a.Description)),
	}
	if a.Message != nil {
		message := fmt.Sprintf("%+v", a.Message)
		message = strings.ReplaceAll(message, "\n", "-")
		attrs = append(attrs, slog.String("message", message))
	}
	return slog.Group("action", attrs...)
}

// LogValue implements slog.LogValuer and returns a grouped value
// with fields redacted. See https://pkg.go.dev/log/slog#LogValuer
func (a Action) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("status", fmt.Sprintf("%d|%s", a.Status.Int(), a.Status.String())),
		slog.String("desc", ToSnakeCase(a.Description)),
	}
	if a.Message != nil {
		message := fmt.Sprintf("%+v", a.Message)
		message = strings.ReplaceAll(message, "\n", "-")
		attrs = append(attrs, slog.String("message", message))
	}
	return slog.GroupValue(attrs...)
}
