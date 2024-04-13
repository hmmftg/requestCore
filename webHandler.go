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
		errData := response.RespData{
			Code:    status,
			Status:  1,
			Message: internalError.Desc,
			Type:    response.Json,
			JSON:    internalError.Message,
		}
		m.Respond(errData, true, w)
		return
	}

	errData := response.RespData{
		Code:    status,
		Status:  1,
		Message: message,
		Type:    response.Json,
		JSON:    data,
	}

	m.Respond(errData, true, w)
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

	errData := response.RespData{
		Code:    err.GetStatus(),
		Status:  1,
		Message: err.GetDescription(),
		Type:    response.Json,
		JSON:    err.GetMessage(),
	}

	m.Respond(errData, true, w)
}

func (m WebHanlder) ErrorState(w webFramework.WebFramework, err response.ErrorState) {
	m.errorState(w, err, 1)
}

func (m WebHanlder) Error(w webFramework.WebFramework, err response.ErrorState) {
	m.errorState(w, err, 1)
}

// func (m WebHanlder) Respond(code, status int, message string, data any, abort bool, w webFramework.WebFramework) {
// 	m.RespondWithReceipt(code, status, message, data, nil, abort, w)
// }

func (m WebHanlder) Respond(data response.RespData, abort bool, w webFramework.WebFramework) {
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
		}
	} else {
		resp.ErrorData = m.GetErrorsArray(data.Message, data)
	}

	err := w.Parser.SendJSONRespBody(data.Code, resp)
	if err != nil {
		log.Println("error in SendJSONRespBody", err)
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

func (m WebHanlder) OK(w webFramework.WebFramework, resp any) {
	data := response.RespData{
		Code:    http.StatusOK,
		Status:  0,
		Message: "OK",
		Type:    response.Json,
		JSON:    resp,
	}

	m.Respond(data, false, w)
}

func (m WebHanlder) OKWithReceipt(w webFramework.WebFramework, resp any, receipt *response.Receipt) {
	data := response.RespData{
		Code:      http.StatusOK,
		Status:    0,
		Message:   "OK",
		Type:      response.JsonWithReceipt,
		JSON:      resp,
		PrintData: receipt,
	}

	m.Respond(data, false, w)
}

func (m WebHanlder) OKWithAttachment(w webFramework.WebFramework, attachment *response.FileResponse) {
	data := response.RespData{
		Code:       http.StatusOK,
		Status:     0,
		Message:    "OK",
		Type:       response.FileAttachment,
		Attachment: attachment,
	}

	m.Respond(data, false, w)
}

func (m WebHanlder) GetErrorsArray(message string, data any) []response.ErrorResponse {
	return response.GetErrorsArrayWithMap(message, data, m.ErrorDesc)
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
