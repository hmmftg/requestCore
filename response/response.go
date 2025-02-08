package response

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
)

const (
	NO_DATA_FOUND     = "NO-DATA-FOUND"
	SYSTEM_FAULT      = "SYSTEM_FAULT"
	SYSTEM_FAULT_DESC = "خطای سیستمی"
)

type ErrorResponse struct {
	Code        string `json:"code"`
	Description any    `json:"description"`
}

type WsRemoteResponse struct {
	Status      int             `json:"status"`
	Description string          `json:"description"`
	Result      any             `json:"result,omitempty"`
	ErrorData   []ErrorResponse `json:"errors,omitempty"`
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

type WsResponse struct {
	Status       int      `json:"status"`
	Description  string   `json:"description"`
	Result       any      `json:"result,omitempty"`
	ErrorData    any      `json:"errors,omitempty"`
	PrintReceipt *Receipt `json:"printReceipt,omitempty"`
}

type Receipt struct {
	Id    string `json:"id"`
	Title string `json:"title"`
	Rows  []any  `json:"rows"`
}

type FileResponse struct {
	FileName string `json:"fileName"`
	Path     string `json:"path"`
}

type DbResponse struct {
	Status      int    `json:"status"`
	Description string `json:"description"`
	Result      any    `json:"result"`
	ErrorCode   string `json:"error_code,omitempty"`
}

type ErrorData struct {
	source      string
	input       any
	Status      int
	Description string
	Message     any
	childs      []ErrorState
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
}

func (e ErrorData) GetStatus() int         { return e.Status }
func (e ErrorData) GetInput() any          { return e.input }
func (e ErrorData) GetDescription() string { return e.Description }
func (e ErrorData) GetMessage() any        { return e.Message }

func (e ErrorData) SetStatus(status int) ErrorState {
	e.Status = status
	return &e
}
func (e ErrorData) SetDescription(desc string) ErrorState {
	e.Description = desc
	return &e
}
func (e ErrorData) SetMessage(msg any) ErrorState {
	e.Message = msg
	return &e
}
func (e ErrorData) ChildErr(err error) ErrorState {
	return e.Child(toErrorState(err, 4))
}
func (e ErrorData) Child(err ErrorState) ErrorState {
	if e.childs == nil {
		e.childs = []ErrorState{err}
	} else {
		e.childs = append(e.childs, err)
	}
	return &e
}
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
	stack.WriteString(fmt.Sprintf("%s%d,%s,%s,%s,%s\n", header, e.Status, e.Description, e.source, jsonInput, jsonMsg))
	childHeader := fmt.Sprintf("%s\t", header)
	for _, errorData := range e.childs {
		switch err := errorData.(type) {
		case *ErrorData:
			err.Format(childHeader, stack)
		}
	}
}

func (e ErrorData) Error() string {
	var stack strings.Builder
	e.Format("", &stack)
	return stack.String()
}

func (e *ErrorData) Input(in any) ErrorState {
	e.input = in
	return e
}

func (e ErrorData) WsResponse() string {
	return fmt.Sprintf("%s#%s#%v#%v#%d", e.Description, e.source, e.input, e.Message, e.Status)
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
	log.Println(errs)
	err := errs.(validator.ValidationErrors)
	errorResponses := make([]ErrorResponse, 0)
	for _, validationError := range err {
		var errorResp ErrorResponse
		path := strings.Split(validationError.Namespace(), ".")
		parent := "Request."

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

func JustPrintResp(respBytes []byte, desc string, status int) (int, map[string]string, any, error) {
	var err error
	var resp WsRemoteResponse
	err = json.Unmarshal(respBytes, &resp)
	if err != nil {
		log.Println(string(respBytes))
	}
	log.Println(resp)
	return status, nil, nil, nil
}

func ParseRemoteRespJson(respBytes []byte, desc string, status int) (int, map[string]string, any, error) {
	var resp WsRemoteResponse
	err := json.Unmarshal(respBytes, &resp)
	if err != nil {
		return http.StatusBadRequest, map[string]string{"desc": "PWC_CICO_0004", "message": err.Error()}, resp, err
	}
	if status != http.StatusOK {
		if len(resp.ErrorData) > 0 {
			errorDesc := resp.ErrorData[0] //.(ErrorResponse)
			errorMessage := errorDesc.Description.(string)
			return status, map[string]string{"desc": errorDesc.Code, "message": errorMessage}, resp, errors.New(errorMessage)
		}
		return status, map[string]string{"desc": "Remote Resp", "message": resp.Description}, resp, errors.New(resp.Description)
	}
	return http.StatusOK, nil, resp.Result, nil
}

func ParseWsRemoteResp(respBytes []byte, desc string, status int) (int, map[string]string, any, error) {
	var resp WsRemoteResponse
	err := json.Unmarshal(respBytes, &resp)
	if err != nil {
		return http.StatusBadRequest, map[string]string{"desc": "PWC_CICO_0004", "message": err.Error()}, resp, err
	}
	if status != http.StatusOK {
		if len(resp.ErrorData) > 0 {
			errorDesc := strings.ReplaceAll(resp.ErrorData[0].Code, "-", "_") //.(ErrorResponse)
			errorMessage := resp.ErrorData[0].Description.(string)
			return status, map[string]string{"desc": errorDesc, "message": errorMessage}, resp, errors.New(errorMessage)
		}
		return status, map[string]string{"desc": "Remote Resp", "message": resp.Description}, resp, errors.New(resp.Description)
	}
	return http.StatusOK, nil, resp, nil
}

func GetErrorsArray(message string, data any) []ErrorResponse {
	var errorResponses []ErrorResponse
	errorResponses, ok := data.([]ErrorResponse)
	if !ok {
		errorResponses = make([]ErrorResponse, 0)
		var errorResp ErrorResponse
		errorResp.Code = message
		errorResp.Description = data
		errorResponses = append(errorResponses, errorResp)
	}
	return errorResponses
}

func GetDescFromCode(code string, data any, errDescList map[string]string) (string, any) {
	if strings.Contains(code, "#") {
		code := code
		if strings.Contains(code, "-") {
			code = strings.ReplaceAll(code, "-", "_")
		}
		messageParts := strings.Split(code, "#")
		if descInDb, ok := errDescList[messageParts[0]]; ok {
			descParts := strings.Split(descInDb, "$")
			incomingDesc := messageParts[0]
			desc := ""
			//DESC_DB1 $P1$ DESC_DB2 $P2$
			//MESSAGE1#G1#G2#
			//=>
			//DESC_DB1 G1 DESC_DB2 G2
			for i, j := 0, 1; i < len(descParts); i += 2 {
				desc += descParts[i] + messageParts[j]
				j++
			}
			return strings.ReplaceAll(incomingDesc, "_", "-"), desc
		}
		return strings.ReplaceAll(code, "_", "-"), data
	}
	if desc, ok := errDescList[code]; ok {
		return strings.ReplaceAll(code, "_", "-"), desc
	}
	return code, data
}

func GetErrorsArrayWithMap(incomingDesc string, data any, errDescList map[string]string) []ErrorResponse {
	var errorResponses []ErrorResponse
	respData, okRespData := data.(RespData)
	if !okRespData {
		log.Printf("GetErrorsArrayWithMap invalid type: %T is not RespData\n", data)
		return nil
	}
	errorResponses, ok := respData.JSON.([]ErrorResponse)
	desc := incomingDesc
	if !ok || len(errorResponses) == 0 {
		errorResponses = make([]ErrorResponse, 0)
		var errorResp ErrorResponse
		if strings.Contains(desc, "-") {
			if respData.JSON != nil { //error already translated
				errorResp.Code = desc
				errorResp.Description = respData.JSON
				return append(errorResponses, errorResp)
			}
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
