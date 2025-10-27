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

type WebHanlder struct {
	MessageDesc map[string]string
	ErrorDesc   map[string]string
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
		webFramework.AddLogTag(w, webFramework.ErrorListLogTag, slog.Any(fmt.Sprintf("error-%d", id), array[id]))
	}
	if newError := getError[libError.ErrorData](err); newError != nil {
		m.Respond(newError.ActionData.Status.Int(), 1, newError.ActionData.Description, newError.ActionData.Message, true, w)
		return
	}
	if oldError := getError[ErrorData](err); oldError != nil {
		m.Respond(oldError.Status, 1, oldError.Description, oldError.Message, true, w)
		return
	}

	webFramework.AddLogTag(w, webFramework.ErrorListLogTag, slog.Any("error", err))
	desc := err.Error()
	desc = strings.ToUpper(desc)
	desc = strings.ReplaceAll(desc, " ", "")
	m.Respond(http.StatusInternalServerError, 1, desc, err, true, w)
}

func (m WebHanlder) Error(w webFramework.WebFramework, err error) {
	m.errorhandler(w, err)
}

func (m WebHanlder) Respond(code, status int, message string, data any, abort bool, w webFramework.WebFramework) {
	m.RespondWithReceipt(code, status, message, data, nil, abort, w)
}

func (m WebHanlder) OK(w webFramework.WebFramework, resp any) {
	m.Respond(http.StatusOK, 0, "OK", resp, false, w)
}

func (m WebHanlder) OKWithReceipt(w webFramework.WebFramework, resp any, receipt *Receipt) {
	m.RespondWithReceipt(http.StatusOK, 0, "OK", resp, receipt, false, w)
}

func (m WebHanlder) OKWithAttachment(w webFramework.WebFramework, attachment *FileResponse) {
	m.RespondWithAttachment(http.StatusOK, 0, "OK", attachment, false, w)
}

func (m WebHanlder) GetErrorsArray(message string, data any) []ErrorResponse {
	return GetErrorsArrayWithMap(message, data, m.ErrorDesc)
}

func (m WebHanlder) RespondWithReceipt(code, status int, message string, data any, printData *Receipt, abort bool, w webFramework.WebFramework) {
	respData := RespData{
		Code:      code,
		Status:    status,
		Message:   message,
		Type:      JsonWithReceipt,
		JSON:      data,
		PrintData: printData,
	}

	m.respond(respData, abort, w)
}

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

	webFramework.AddLogTag(w, webFramework.HandlerLogTag, slog.Int("status", data.Code))
	if data.Code == http.StatusOK {
		resp.Description = m.MessageDesc[data.Message]
		switch data.Type {
		case FileAttachment:
			w.Parser.FileAttachment(data.Attachment.Path, data.Attachment.FileName)
		case JsonWithReceipt:
			resp.PrintReceipt = data.PrintData
			fallthrough
		case Json:
			resp.Result = data.JSON

			err := w.Parser.SendJSONRespBody(data.Code, resp)
			if err != nil {
				webFramework.AddLog(w, webFramework.HandlerLogTag,
					slog.Group("error in SendJSONRespBody", slog.Any("error", err)))
			}
		}
	} else {
		errs := m.GetErrorsArray(data.Message, data)
		if len(errs) == 1 {
			webFramework.AddLogTag(w, webFramework.HandlerLogTag, slog.String("desc", errs[0].Code))
			if strMsg, ok := errs[0].Description.(string); ok {
				webFramework.AddLogTag(w, webFramework.HandlerLogTag, slog.String("message", strMsg))
			} else {
				webFramework.AddLogTag(w, webFramework.HandlerLogTag, slog.Any("message", errs[0].Description))
			}
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
