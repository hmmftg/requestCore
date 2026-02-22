// Package response provides HTTP response and error handling for requestCore.
//
// Safe error response (for consuming repos):
//   - Do not put string(rawResp) or full upstream response bodies in libError descriptions;
//     use size/status/hash and log details separately (see libCallApi).
//   - Do not use %+v on whole response structs in error messages that can become client-visible.
//   - Use only codes from a fixed catalog and ensure they are seeded/localized (e.g. SYSTEM_FAULT, API_*);
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

type ErrorResponse struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

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

func ToErrorState(err error) ErrorState {
	return toErrorState(err, 2)
}

func ToError(desc string, message any, err error) ErrorState {
	return Error(http.StatusInternalServerError, desc, message, err)
}

func Error(status int, desc string, message any, err error) ErrorState {
	return Errors(status, desc, message, toErrorState(err, 3))
}

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
