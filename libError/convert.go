package libError

import (
	"fmt"
	"strings"
	"time"

	"github.com/hmmftg/requestCore/status"
)

func Convert(err error, status status.StatusCode, desc string, message any) Error {
	return ErrorData{
		Time:       time.Now(),
		Source:     getStack(),
		ActionData: action(status, desc, message),
		Child:      convert(err, nil),
	}
}

func action(status status.StatusCode, desc string, message any) Action {
	return Action{status, desc, message}
}

// wraps child error with parent status code and desc
func Add(err Error, status status.StatusCode, desc string, message any) Error {
	return ErrorData{
		Time:       time.Now(),
		Source:     getStack(),
		ActionData: action(status, desc, message),
		Child:      err,
	}
}

// creates new error object
func New(status status.StatusCode, desc string, format string, a ...any) Error {
	return ErrorData{
		Time:       time.Now(),
		ActionData: action(status, desc, fmt.Sprintf(format, a...)),
		Source:     getStack(),
	}
}

func convert(err error, src *Source) Error {
	return &ErrorData{
		Time:       time.Now(),
		ActionData: action(status.Unknown, "INTERNAL_ERROR", err),
		Source:     src,
	}
}

func (e ErrorData) Error() string {
	stack := strings.Builder{}
	e.Format(&stack)
	return stack.String()
}
