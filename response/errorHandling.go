package response

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"strings"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
)

type ErrorResponse struct {
	Code        string `json:"code"`
	Description any    `json:"description"`
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

		switch validationtag[0] {
		case "required", "required_unless", "required_if":
			errorResp.Code = "REQUIRED-FIELD"
		case "padded_ip":
			fallthrough
		case "startswith":
			fallthrough
		case "alphanum":
			fallthrough
		case "oneof":
			fallthrough
		case "numeric":
			fallthrough
		case "len":
			fallthrough
		case "min":
			fallthrough
		case "max":
			errorResp.Code = "INVALID-INPUT-DATA"
		default:
			parent += validationError.Tag()
			errorResp.Code = "INVALID-INPUT-DATA"
		}
		// complicated tag
		if len(validationtag) > 1 {
			errorResp.Description = fmt.Sprintf("%s فیلد %s اجباری میباشد", parent, validationError.Field())
		} else {
			errorResp.Description = parent + " " + validationError.Translate(trans)
		}
		errorResponses = append(errorResponses, errorResp)
	}
	return errorResponses
}
