package libError

import (
	"fmt"
	"strings"
	"time"

	"github.com/hmmftg/requestCore/status"
)

// Convert wraps a standard error into a structured Error with the given status, description, and message.
func Convert(err error, status status.StatusCode, desc string, message any) Error {
	return ErrorData{
		Time:       time.Now(),
		Source:     getStack(),
		ActionData: action(status, desc, message),
		Child:      convert(err, nil),
	}
}

func action(status status.StatusCode, desc string, message any) Action {
	return Action{Status: status, Description: desc, Message: message}
}

// Add wraps the given error with a parent status code and description.
func Add(err Error, status status.StatusCode, desc string, message any) Error {
	return ErrorData{
		Time:       time.Now(),
		Source:     getStack(),
		ActionData: action(status, desc, message),
		Child:      err,
	}
}

// NewWithDescription creates a new Error with the given status, description, and formatted message.
func NewWithDescription(status status.StatusCode, desc string, format string, a ...any) Error {
	return ErrorData{
		Time:       time.Now(),
		ActionData: action(status, desc, fmt.Sprintf(format, a...)),
		Source:     getStack(),
	}
}

// New creates a new Error with the given status, description, and message.
func New(status status.StatusCode, desc string, msg any) Error {
	return ErrorData{
		Time:       time.Now(),
		ActionData: action(status, desc, msg),
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

// Error returns the formatted string representation of the full error chain.
func (e ErrorData) Error() string {
	stack := strings.Builder{}
	e.Format(&stack)
	return stack.String()
}
