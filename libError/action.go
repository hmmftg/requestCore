package libError

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/hmmftg/requestCore/status"
)

type Action struct {
	Status      status.StatusCode `json:"status"`
	Description string            `json:"description"`
	Message     any               `json:"message"`
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
		"status: %s, desc: %s",
		a.Status.String(),
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
		slog.String("status", a.Status.String()),
		slog.String("desc", ToSnakeCase(a.Description)),
	}
	if a.Message != nil {
		message := fmt.Sprintf("%+v", a.Message)
		message = strings.ReplaceAll(message, "\n", "-")
		attrs = append(attrs, slog.String("message", message))
	}
	return slog.Group("action", attrs...)
}