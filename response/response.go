package response

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
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

func (e WsRemoteResponse) ToErrorState() *ErrorState {
	if len(e.Description) > 0 {
		if len(e.ErrorData) == 0 {
			return &ErrorState{
				Description: e.Description,
				Status:      e.Status,
				Message:     e.Result,
				ErrorDesc:   e.Description,
			}
		}
		return &ErrorState{
			Description: e.Description,
			Status:      e.Status,
			Message:     e.ErrorData,
			ErrorDesc:   e.Description,
		}
	}
	lastId := len(e.ErrorData) - 1
	var errDesc string
	switch casted := e.ErrorData[0].Description.(type) {
	case string:
		errDesc = casted
	}
	return &ErrorState{
		Description: e.ErrorData[0].Code,
		Status:      e.Status,
		Message:     e.ErrorData,
		ErrorDesc:   errDesc,
		Child:       fmt.Errorf("%s#%s", e.ErrorData[lastId].Code, e.ErrorData[lastId].Description),
	}
}

type WsResponse struct {
	Status       int     `json:"status"`
	Description  string  `json:"description"`
	Result       any     `json:"result,omitempty"`
	ErrorData    any     `json:"errors,omitempty"`
	PrintReceipt Receipt `json:"printReceipt,omitempty"`
}

type Receipt struct {
	Id    string `json:"id"`
	Title string `json:"title"`
	Rows  []any  `json:"rows"`
}

type DbResponse struct {
	Status      int    `json:"status"`
	Description string `json:"description"`
	Result      any    `json:"result"`
	ErrorCode   string `json:"error_code,omitempty"`
}

type ErrorState struct {
	Source      string
	Input       any
	ErrorDesc   string
	Status      int
	Description string
	Message     any
	Child       error
}

func (e ErrorState) Error() string {
	return e.ErrorDesc
}

func (e ErrorState) AddSource(src string) *ErrorState {
	e.Source = src
	return &e
}

func (e ErrorState) AddInput(in any) *ErrorState {
	e.Input = in
	return &e
}

func (e ErrorState) WsResponse() string {
	return fmt.Sprintf("%s#%s#%s#%v#%v#%d", e.Description, e.ErrorDesc, e.Source, e.Input, e.Message, e.Status)
}

func ToErrorState(err error) *ErrorState {
	return &ErrorState{
		ErrorDesc: err.Error(),
		Child:     err,
	}
}

func ToError(desc string, message any, err error) *ErrorState {
	return &ErrorState{
		ErrorDesc:   err.Error(),
		Child:       err,
		Description: desc,
		Message:     message,
	}
}

func Error(status int, desc string, message any, err error) *ErrorState {
	return &ErrorState{
		ErrorDesc:   err.Error(),
		Child:       err,
		Description: desc,
		Message:     message,
		Status:      status,
	}
}

func FormatErrorResp(errs error, trans ut.Translator) []ErrorResponse {
	log.Println(errs)
	err := errs.(validator.ValidationErrors)
	errorResponses := make([]ErrorResponse, 0)
	for _, validationError := range err {
		var errorResp ErrorResponse
		path := strings.Split(validationError.StructNamespace(), ".")
		parent := ""

		if path[0] == "RequestHeader" {
			parent = "Header"
		}

		if len(path) > 2 {
			for i := 1; i < len(path)-1; i++ {
				parent = parent + path[i] + "."
			}
		}

		switch validationError.Tag() {
		case "required":
			errorResp.Code = "REQUIRED-FIELD"
		case "padded_ip":
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
		errorResp.Description = parent + " " + validationError.Translate(trans)
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
	errorResponses, ok := data.([]ErrorResponse)
	if !ok || len(errorResponses) == 0 {
		errorResponses = make([]ErrorResponse, 0)
		var errorResp ErrorResponse
		if strings.Contains(incomingDesc, "-") { //error already translated
			errorResp.Code = incomingDesc
			errorResp.Description = data
			return append(errorResponses, errorResp)
		}
		errorResp.Code, errorResp.Description = GetDescFromCode(incomingDesc, data, errDescList)
		errorResponses = append(errorResponses, errorResp)
	}
	for i := 0; i < len(errorResponses); i++ {
		if !strings.Contains(errorResponses[i].Code, "-") || strings.Contains(errorResponses[i].Code, "#") {
			errorResponses[i].Code, errorResponses[i].Description = GetDescFromCode(incomingDesc, data, errDescList)
		}
	}
	return errorResponses
}
