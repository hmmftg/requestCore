package response

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/webFramework"
)

// LastHTTPStatusLocal is the local-storage key for the most recent HTTP status code.
const LastHTTPStatusLocal = "response.http_status"

// WebHanlder holds description lookup maps for building API error responses.
type WebHanlder struct {
	MessageDesc map[string]string
	ErrorDesc   map[string]string
}

// LastHTTPStatus returns the HTTP status code recorded by the most recent respond call.
func LastHTTPStatus(w webFramework.WebFramework) int {
	if v := w.Parser.GetLocal(LastHTTPStatusLocal); v != nil {
		if code, ok := v.(int); ok {
			return code
		}
	}
	return 0
}

func getError[Result error](err error) *Result {
	newError := new(Result)
	if errors.As(err, newError) {
		return newError
	}
	return nil
}

func getList(err error) []error {
	list := []error{err}
	child := errors.Unwrap(err)
	for child != nil {
		list = append(list, child)
		child = errors.Unwrap(child)
	}
	return list
}

func (m WebHanlder) errorhandler(w webFramework.WebFramework, err error) {
	array := getList(err)
	for id := range array {
		webFramework.AddLogTag(w, webFramework.ErrorListLogTag, slog.String(fmt.Sprintf("error-%d", id), array[id].Error()))
	}
	if newError := getError[libError.ErrorData](err); newError != nil {
		// Do not pass Action.Message to response. Use PublicDescription when set; otherwise description from ErrorDesc/safe fallback.
		if newError.ActionData.PublicDescription != "" {
			errs := []ErrorResponse{{
				Code:        newError.ActionData.Description,
				Description: SanitizeForClient(newError.ActionData.PublicDescription, MaxDescriptionLength),
			}}
			m.respond(RespData{
				Code: newError.ActionData.Status.Int(), Status: 1, Message: newError.ActionData.Description,
				Type: JSONWithReceipt, PreBuiltErrors: &errs,
			}, true, w)
			return
		}
		if errs, ok := newError.ActionData.Message.([]ErrorResponse); ok && len(errs) > 0 {
			m.Respond(newError.ActionData.Status.Int(), 1, newError.ActionData.Description, errs, true, w)
			return
		}
		m.Respond(newError.ActionData.Status.Int(), 1, newError.ActionData.Description, nil, true, w)
		return
	}
	if oldError := getError[ErrorData](err); oldError != nil {
		// Do not pass Message to response — only code; description from ErrorDesc/safe fallback.
		m.Respond(oldError.Status, 1, oldError.Description, nil, true, w)
		return
	}

	webFramework.AddLogTag(w, webFramework.ErrorListLogTag, slog.String("error", err.Error()))
	desc := err.Error()
	desc = strings.ToUpper(desc)
	desc = strings.ReplaceAll(desc, " ", "")
	// Do not pass raw err to response; use code only so description is resolved from ErrorDesc/safe fallback.
	m.Respond(http.StatusInternalServerError, 1, desc, nil, true, w)
}

// Error handles an error by logging it and sending an appropriate error response to the client.
func (m WebHanlder) Error(w webFramework.WebFramework, err error) {
	m.errorhandler(w, err)
}

// Respond sends a JSON response with the given code, status, message, and data, optionally aborting the request chain.
func (m WebHanlder) Respond(code, status int, message string, data any, abort bool, w webFramework.WebFramework) {
	m.RespondWithReceipt(code, status, message, data, nil, abort, w)
}

// OK sends a successful JSON response with HTTP 200 and the given data.
func (m WebHanlder) OK(w webFramework.WebFramework, resp any) {
	m.Respond(http.StatusOK, 0, "OK", resp, false, w)
}

// OKWithStatus sends a successful JSON response with the given HTTP
// status and data. The status must be a valid 2xx code. For 204 and
// 205, the response body is suppressed (no JSON is written). This is
// an additive, opt-in method; the default OK continues to use 200.
func (m WebHanlder) OKWithStatus(w webFramework.WebFramework, status int, resp any) {
	m.Respond(status, 0, "OK", resp, false, w)
}

// OKWithStatusAndHeaders sends a successful JSON response with the
// given HTTP status, static response headers, and data. Headers are
// applied before the body is written. For 204 and 205, the body is
// suppressed but headers are still applied.
func (m WebHanlder) OKWithStatusAndHeaders(w webFramework.WebFramework, status int, headers map[string]string, resp any) {
	for k, v := range headers {
		w.Parser.SetRespHeader(k, v)
	}
	m.Respond(status, 0, "OK", resp, false, w)
}

// OKWithReceipt sends a successful JSON response with an optional printable receipt.
func (m WebHanlder) OKWithReceipt(w webFramework.WebFramework, resp any, receipt *Receipt) {
	m.RespondWithReceipt(http.StatusOK, 0, "OK", resp, receipt, false, w)
}

// OKWithAttachment sends a successful file-attachment response with HTTP 200.
func (m WebHanlder) OKWithAttachment(w webFramework.WebFramework, attachment *FileResponse) {
	m.RespondWithAttachment(http.StatusOK, 0, "OK", attachment, false, w)
}

// GetErrorsArray builds a slice of ErrorResponse from a message and data using the handler's error description map.
func (m WebHanlder) GetErrorsArray(message string, data any) []ErrorResponse {
	return GetErrorsArrayWithMap(message, data, m.ErrorDesc)
}

// RespondWithReceipt sends a JSON response with an optional printable receipt and abort control.
func (m WebHanlder) RespondWithReceipt(code, status int, message string, data any, printData *Receipt, abort bool, w webFramework.WebFramework) {
	respData := RespData{
		Code:      code,
		Status:    status,
		Message:   message,
		Type:      JSONWithReceipt,
		JSON:      data,
		PrintData: printData,
	}

	m.respond(respData, abort, w)
}

// RespondWithAttachment sends a file-attachment response with the given code, status, and message.
func (m WebHanlder) RespondWithAttachment(code, status int, message string, file *FileResponse, abort bool, w webFramework.WebFramework) {
	respData := RespData{
		Code:       code,
		Status:     status,
		Message:    message,
		Type:       FileAttachment,
		Attachment: file,
	}

	m.respond(respData, abort, w)
}

func (m WebHanlder) respond(data RespData, abort bool, w webFramework.WebFramework) {
	var resp WsResponse
	resp.Status = data.Status

	w.Parser.SetLocal(LastHTTPStatusLocal, data.Code)
	webFramework.AddLogTag(w, webFramework.HandlerLogTag, slog.Int("status", data.Code))
	if data.Status == 0 {
		// Success path: status 0 means success. This allows any 2xx
		// HTTP status to be used for success responses, not just 200.
		resp.Description = m.MessageDesc[data.Message]
		switch data.Type {
		case FileAttachment:
			w.Parser.FileAttachment(data.Attachment.Path, data.Attachment.FileName)
		case JSONWithReceipt:
			resp.PrintReceipt = data.PrintData
			fallthrough
		case JSON:
			// Suppress body for 204 No Content and 205 Reset Content
			// per RFC 9110. A 204/205 response must not have a body.
			if data.Code == http.StatusNoContent || data.Code == http.StatusResetContent {
				// Send only the status code without a body. Use
				// SendJSONRespBody with nil to set the status without
				// writing JSON content.
				err := w.Parser.SendJSONRespBody(data.Code, nil)
				if err != nil {
					webFramework.AddLog(w, webFramework.HandlerLogTag,
						slog.Group("error in SendJSONRespBody", slog.Any("error", err)))
				}
			} else {
				resp.Result = data.JSON
				err := w.Parser.SendJSONRespBody(data.Code, resp)
				if err != nil {
					webFramework.AddLog(w, webFramework.HandlerLogTag,
						slog.Group("error in SendJSONRespBody", slog.Any("error", err)))
				}
			}
		}
	} else {
		var errs []ErrorResponse
		if data.PreBuiltErrors != nil {
			errs = *data.PreBuiltErrors
		} else {
			errs = m.GetErrorsArray(data.Message, data)
		}
		if len(errs) == 1 {
			webFramework.AddLogTag(w, webFramework.HandlerLogTag, slog.String("desc", errs[0].Code))
			webFramework.AddLogTag(w, webFramework.HandlerLogTag, slog.String("message", errs[0].Description))
		} else {
			webFramework.AddLogTag(w, webFramework.HandlerLogTag, slog.Any("errorList", errs))
		}
		resp.ErrorData = errs

		w.Parser.SetLocal("errorArray", resp.ErrorData)

		err := w.Parser.SendJSONRespBody(data.Code, resp)
		if err != nil {
			webFramework.AddLog(w, webFramework.HandlerLogTag,
				slog.Group("error in SendJSONRespBody", slog.Any("error", err)))
		}
	}

	if abort {
		err := w.Parser.Abort()
		if err != nil {
			webFramework.AddLog(w, webFramework.HandlerLogTag,
				slog.Group("error in Abort", slog.Any("error", err)))
		}
	} else {
		err := w.Parser.Next()
		if err != nil {
			webFramework.AddLog(w, webFramework.HandlerLogTag,
				slog.Group("error in Next", slog.Any("error", err)))
		}
	}
}
