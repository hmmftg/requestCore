package response

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// ErrorData holds structured error information including status, description, message, and child errors.
type ErrorData struct {
	source      string
	input       any
	Status      int
	Description string
	Message     any
	childs      []ErrorState
}

// Unwrap attempts to extract an ErrorState from the given error, returning whether it succeeded.
func Unwrap(err error) (bool, ErrorState) {
	errData := &ErrorData{}
	if errors.As(err, errData) {
		return true, errData
	}
	return false, nil
}

// LogValue implements slog.LogValuer and returns a grouped value
// with fields redacted. See https://pkg.go.dev/log/slog#LogValuer
func (e ErrorData) LogValue() slog.Value {
	children := []any{}
	for id := range e.childs {
		children = append(children, slog.Any(e.childs[id].GetDescription(), e.childs[id]))
	}
	return slog.GroupValue(
		slog.Int("status", e.Status),
		slog.String("desc", e.Description),
		slog.Any("message", e.Message),
		slog.Any("source", e.source),
		slog.Group("children", children...),
	)
}

// GetStatus returns the HTTP status code associated with the error.
func (e ErrorData) GetStatus() int { return e.Status }

// GetInput returns the input data associated with the error.
func (e ErrorData) GetInput() any { return e.input }

// GetDescription returns the error description string.
func (e ErrorData) GetDescription() string { return e.Description }

// GetMessage returns the error message payload.
func (e ErrorData) GetMessage() any { return e.Message }

// SetStatus sets the HTTP status code and returns the updated ErrorState.
func (e ErrorData) SetStatus(status int) ErrorState {
	e.Status = status
	return &e
}

// SetDescription sets the error description and returns the updated ErrorState.
func (e ErrorData) SetDescription(desc string) ErrorState {
	e.Description = desc
	return &e
}

// SetMessage sets the error message payload and returns the updated ErrorState.
func (e ErrorData) SetMessage(msg any) ErrorState {
	e.Message = msg
	return &e
}

// ChildErr wraps a standard error as a child ErrorState and appends it to the chain.
func (e ErrorData) ChildErr(err error) ErrorState {
	return e.Child(toErrorState(err, 4))
}

// Child appends an ErrorState as a child to this error and returns the updated state.
func (e ErrorData) Child(err ErrorState) ErrorState {
	if e.childs == nil {
		e.childs = []ErrorState{err}
	} else {
		e.childs = append(e.childs, err)
	}
	return &e
}

// Format writes a structured text representation of the error and its children into the given builder.
func (e ErrorData) Format(header string, stack *strings.Builder) {
	var jsonMsg, jsonInput string
	if e.input != nil {
		js, _ := json.Marshal(e.input)
		jsonInput = string(js)
	}
	if e.Message != nil {
		js, _ := json.Marshal(e.Message)
		jsonMsg = string(js)
	}
	fmt.Fprintf(stack, "%s%d,%s,%s,%s,%s\n", header, e.Status, e.Description, e.source, jsonInput, jsonMsg)
	childHeader := fmt.Sprintf("%s\t", header)
	for _, errorData := range e.childs {
		switch err := errorData.(type) {
		case *ErrorData:
			err.Format(childHeader, stack)
		}
	}
}

// Error returns a formatted multi-line string representation of the error chain.
func (e ErrorData) Error() string {
	var stack strings.Builder
	e.Format("", &stack)
	return stack.String()
}

// Input associates input data with the error and returns the updated ErrorState.
func (e *ErrorData) Input(in any) ErrorState {
	e.input = in
	return e
}

// WsResponse returns a hash-like string combining the error's description, source, input, message, and status.
func (e ErrorData) WsResponse() string {
	return fmt.Sprintf("%s#%s#%v#%v#%d", e.Description, e.source, e.input, e.Message, e.Status)
}

// GetErrorsArray builds a slice of ErrorResponse from a message and data, handling pre-built error arrays.
func GetErrorsArray(message string, data any) []ErrorResponse {
	var errorResponses []ErrorResponse
	errorResponses, ok := data.([]ErrorResponse)
	if !ok {
		errorResponses = make([]ErrorResponse, 0)
		var errorResp ErrorResponse
		errorResp.Code = message
		errorResp.Description = SanitizeForClient(data, MaxDescriptionLength)
		errorResponses = append(errorResponses, errorResp)
	}
	return errorResponses
}

// GetErrorsArrayWithMap resolves error codes against a description map to produce localized ErrorResponse entries.
func GetErrorsArrayWithMap(incomingDesc string, data any, errDescList map[string]string) []ErrorResponse {
	var errorResponses []ErrorResponse
	respData, okRespData := data.(RespData)
	if !okRespData {
		slog.Error("GetErrorsArrayWithMap invalid type", slog.String("info", fmt.Sprintf("%T is not RespData", data)))
		return nil
	}
	errorResponses, ok := respData.JSON.([]ErrorResponse)
	desc := incomingDesc
	if !ok || len(errorResponses) == 0 {
		errorResponses = make([]ErrorResponse, 0)
		var errorResp ErrorResponse
		if strings.Contains(desc, "-") {
			desc = strings.ReplaceAll(desc, "-", "_")
		}
		errorResp.Code, errorResp.Description = GetDescFromCode(desc, respData.JSON, errDescList)
		errorResponses = append(errorResponses, errorResp)
	}
	for i := 0; i < len(errorResponses); i++ {
		if !strings.Contains(errorResponses[i].Code, "-") || strings.Contains(errorResponses[i].Code, "#") {
			errorResponses[i].Code, errorResponses[i].Description = GetDescFromCode(errorResponses[i].Code, errorResponses[i].Description, errDescList)
		}
	}
	return errorResponses
}
