package requestCore

import (
	"fmt"
	"log"
	"net/http"

	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

type WebHanlder struct {
	MessageDesc      map[string]string
	ErrorDesc        map[string]string
	RequestInterface libRequest.RequestInterface
}

func (m WebHanlder) HandleErrorState(err error, status int, message string, data any, w webFramework.WebFramework) {
	log.Printf("error state: %+v", err)

	if r := w.Parser.GetLocal("reqLog"); r != nil {
		reqLog := r.(libRequest.RequestPtr)
		m.RequestInterface.LogEnd("HandleError", fmt.Sprintf("desc: %s, err: %v, data: %v", message, err, data), reqLog)
	}
	switch internalError := err.(type) {
	case response.InternalError:
		m.Respond(status, 1, internalError.Desc, internalError.Message, true, w)
		return
	}
	m.Respond(status, 1, message, data, true, w)
}

func (m WebHanlder) errorState(w webFramework.WebFramework, err response.ErrorState, depth int) {
	src := response.GetStack(depth+1, "requestCore/webHandler.go")
	log.Printf("error state: %s, \n%+v", src, err)

	if r := w.Parser.GetLocal("reqLog"); r != nil {
		reqLog := r.(libRequest.RequestPtr)
		m.RequestInterface.LogEnd(
			"HandleError",
			fmt.Sprintf("desc: %s, err: %v, data: %v", err.GetDescription(), err, err.GetMessage()),
			reqLog,
		)
	}

	m.Respond(err.GetStatus(), 1, err.GetDescription(), err.GetMessage(), true, w)
}

func (m WebHanlder) ErrorState(w webFramework.WebFramework, err response.ErrorState) {
	m.errorState(w, err, 1)
}

func (m WebHanlder) Error(w webFramework.WebFramework, err response.ErrorState) {
	m.errorState(w, err, 1)
}

func (m WebHanlder) Respond(code, status int, message string, data any, abort bool, w webFramework.WebFramework) {
	m.RespondWithReceipt(code, status, message, data, nil, abort, w)
}

func (m WebHanlder) OK(w webFramework.WebFramework, resp any) {
	m.Respond(http.StatusOK, 0, "OK", resp, false, w)
}

func (m WebHanlder) OKWithReceipt(w webFramework.WebFramework, resp any, receipt *response.Receipt) {
	m.RespondWithReceipt(http.StatusOK, 0, "OK", resp, receipt, false, w)
}

func (m WebHanlder) OKWithAttachment(w webFramework.WebFramework, attachment *response.FileResponse) {
	m.RespondWithAttachment(http.StatusOK, 0, "OK", attachment, false, w)
}

func (m WebHanlder) GetErrorsArray(message string, data any) []response.ErrorResponse {
	return response.GetErrorsArrayWithMap(message, data, m.ErrorDesc)
}

func (m WebHanlder) RespondWithReceipt(code, status int, message string, data any, printData *response.Receipt, abort bool, w webFramework.WebFramework) {
	respData := response.RespData{
		Code:      code,
		Status:    status,
		Message:   message,
		Type:      response.JsonWithReceipt,
		JSON:      data,
		PrintData: printData,
	}

	m.respond(respData, abort, w)
}

func (m WebHanlder) RespondWithAttachment(code, status int, message string, file *response.FileResponse, abort bool, w webFramework.WebFramework) {
	respData := response.RespData{
		Code:       code,
		Status:     status,
		Message:    message,
		Type:       response.FileAttachment,
		Attachment: file,
	}

	m.respond(respData, abort, w)
}

func (m WebHanlder) respond(data response.RespData, abort bool, w webFramework.WebFramework) {
	var resp response.WsResponse
	resp.Status = data.Status
	if data.Code == http.StatusOK {
		resp.Description = m.MessageDesc[data.Message]
		switch data.Type {
		case response.FileAttachment:
			w.Parser.FileAttachment(data.Attachment.Path, data.Attachment.FileName)
		case response.JsonWithReceipt:
			resp.PrintReceipt = data.PrintData
			fallthrough
		case response.Json:
			resp.Result = data.JSON

			err := w.Parser.SendJSONRespBody(data.Code, resp)
			if err != nil {
				log.Println("error in SendJSONRespBody", err)
			}
		}
	} else {
		resp.ErrorData = m.GetErrorsArray(data.Message, data)

		err := w.Parser.SendJSONRespBody(data.Code, resp)
		if err != nil {
			log.Println("error in SendJSONRespBody", err)
		}
	}

	if r := w.Parser.GetLocal("reqLog"); r != nil {
		reqLog := r.(libRequest.RequestPtr)
		reqLog.Id = w.Parser.GetHeaderValue("Request-Id")
		reqLog.BranchId = w.Parser.GetHeaderValue("Branch-Id")
		if len(data.Message) > 63 {
			reqLog.Result = data.Message[:63]
		} else {
			reqLog.Result = data.Message
		}

		reqLog.Outgoing = resp //string(respB)
		if data.Message != "DUPLICATE_REQUEST" {
			err := m.RequestInterface.UpdateRequestWithContext(w.Ctx, reqLog)
			if err != nil {
				log.Println("error in UpdateRequest", err)
			}
		}
	}

	if abort {
		err := w.Parser.Abort()
		if err != nil {
			log.Println("error in Abort", err)
		}
	} else {
		err := w.Parser.Next()
		if err != nil {
			log.Println("error in Next", err)
		}
	}
}

func Respond(code, status int, message string, data any, abort bool, w webFramework.WebFramework) {
	var resp response.WsResponse
	resp.Status = status
	if code == 200 {
		resp.Description = message
		resp.Result = data
	} else {
		resp.ErrorData = response.GetErrorsArray(message, data)
	}

	err := w.Parser.SendJSONRespBody(code, resp)
	if err != nil {
		log.Println("error in SendJSONRespBody", err)
	}
	if abort {
		err = w.Parser.Abort()
		if err != nil {
			log.Println("error in Abort", err)
		}
	} else {
		err = w.Parser.Next()
		if err != nil {
			log.Println("error in Next", err)
		}
	}
}
