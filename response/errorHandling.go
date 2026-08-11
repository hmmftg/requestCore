// Package response provides HTTP response and error handling for requestCore.
//
// Safe error response (for consuming repos):
//   - Do not put string(rawResp) or full upstream response bodies in libError descriptions;
//     use size/status/hash and log details separately (see libCallApi).
//   - Do not use %+v on whole response structs in error messages that can become client-visible.
//   - Use only codes from a fixed catalog and ensure they are seeded/localized (e.g. SystemFault, API_*);
//     avoid dynamic values as public error codes.
package response

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"strings"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"

	"github.com/hmmftg/requestCore/libValidate"
)

// ErrorResponse represents a single error entry returned to API clients.
type ErrorResponse struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

// ToErrorState converts a WsRemoteResponse into an ErrorState for error chaining.
func (e WsRemoteResponse) ToErrorState() ErrorState {
	if len(e.Description) > 0 {
		if len(e.ErrorData) == 0 {
			return &ErrorData{
				Description: e.Description,
				Status:      e.Status,
				Message:     e.Result,
			}
		}
		return &ErrorData{
			Description: e.Description,
			Status:      e.Status,
			Message:     e.ErrorData,
		}
	}
	return &ErrorData{
		Description: e.ErrorData[0].Code,
		Status:      e.Status,
		Message:     e.ErrorData,
	}
}

// ErrorState defines the interface for structured error data with chaining and logging support.
type ErrorState interface {
	Error() string
	Input(in any) ErrorState
	GetInput() any
	WsResponse() string
	SetStatus(int) ErrorState
	SetDescription(string) ErrorState
	SetMessage(any) ErrorState
	ChildErr(error) ErrorState
	Child(ErrorState) ErrorState
	GetStatus() int
	GetDescription() string
	GetMessage() any
	LogValue() slog.Value
}

// GetStack returns a formatted caller stack frame, skipping framework-internal files.
func GetStack(skip int, exclude string) string {
	_, filename, line, _ := runtime.Caller(skip + 1)
	localSkip := skip
	for strings.Contains(filename, "requestCore/response/response.go") ||
		strings.Contains(filename, exclude) {
		localSkip++
		_, filename, line, _ = runtime.Caller(localSkip)
	}
	return fmt.Sprintf("%s:%d", filename, line)
}

func toErrorState(err error, skip int) ErrorState {
	src := GetStack(skip, "requestCore/response/response.go")
	return &ErrorData{
		Description: err.Error(),
		source:      src,
	}
}

// ToErrorState converts a standard error into an ErrorState with source tracking.
func ToErrorState(err error) ErrorState {
	return toErrorState(err, 2)
}

// ToError creates an ErrorState with an internal-server-error status from the given description, message, and error.
func ToError(desc string, message any, err error) ErrorState {
	return Error(http.StatusInternalServerError, desc, message, err)
}

// Error creates an ErrorState with the given status, description, message, and wrapped error.
func Error(status int, desc string, message any, err error) ErrorState {
	return Errors(status, desc, message, toErrorState(err, 3))
}

// Errors creates an ErrorState by chaining an existing ErrorState with a new status and description.
func Errors(status int, desc string, message any, err ErrorState) ErrorState {
	_, filename, line, _ := runtime.Caller(1)
	src := fmt.Sprintf("%s:%d", filename, line)
	return ErrorData{
		Description: desc,
		Message:     message,
		Status:      status,
		source:      src,
	}.Child(err)
}

// FormatErrorResp translates validator validation errors into a slice of ErrorResponse using the given translator.
func FormatErrorResp(errs error, trans ut.Translator) []ErrorResponse {
	err := errs.(validator.ValidationErrors)
	errorResponses := make([]ErrorResponse, 0)
	for _, validationError := range err {
		var errorResp ErrorResponse
		path := strings.Split(validationError.Namespace(), ".")
		parent := "."

		if path[0] == "RequestHeader" {
			parent = "Header."
		}

		if len(path) > 2 {
			for i := 1; i < len(path)-1; i++ {
				parent = parent + path[i] + "."
			}
		}
		parent = parent[:len(parent)-1]

		validationtag := strings.Split(validationError.Tag(), "=")
		tagName := validationtag[0]

		// Get error code from libValidate registry
		errorResp.Code = libValidate.GetErrorCode(tagName)

		// Check if it's a custom validator
		isCustomValidator := libValidate.IsCustomValidator(tagName)

		// complicated tag
		if len(validationtag) > 1 {
			errorResp.Description = fmt.Sprintf("%s فیلد %s اجباری میباشد", parent, validationError.Field())
		} else {
			translatedMsg := validationError.Translate(trans)
			if isCustomValidator {
				// Custom validator: use translation directly without parent prefix
				// The translation already contains the field name and proper message
				errorResp.Description = translatedMsg
			} else {
				// Known validators: keep existing behavior with parent prefix
				errorResp.Description = parent + " " + translatedMsg
			}
		}
		errorResponses = append(errorResponses, errorResp)
	}
	return errorResponses
}
