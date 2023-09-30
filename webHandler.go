package requestCore

import (
	"fmt"
	"log"

	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
	"github.com/hmmftg/stacktrace"
)

type WebHanlder struct {
	MessageDesc      map[string]string
	ErrorDesc        map[string]string
	RequestInterface libRequest.RequestInterface
}

func (m WebHanlder) HandleErrorState(err error, status int, message string, data any, w webFramework.WebFramework) {
	log.Println(err.Error(), stacktrace.PropagateWithDepth(err, 1, ""))

	if r := w.Parser.GetLocal("reqLog"); r != nil {
		reqLog := r.(*libRequest.Request)
		m.RequestInterface.LogEnd("HandleError", fmt.Sprintf("desc: %s, err: %v, data: %v", message, err, data), reqLog)
	}
	switch internalError := err.(type) {
	case response.InternalError:
		m.Respond(status, 1, internalError.Desc, internalError.Message, true, w)
		return
	}
	m.Respond(status, 1, message, data, true, w)
}

func (m WebHanlder) ErrorState(w webFramework.WebFramework, err *response.ErrorState) {
	log.Println(err.Error(), stacktrace.PropagateWithDepth(err, 1, ""))

	if r := w.Parser.GetLocal("reqLog"); r != nil {
		reqLog := r.(*libRequest.Request)
		m.RequestInterface.LogEnd("HandleError", fmt.Sprintf("desc: %s, err: %v, data: %v", err.Description, err, err.Message), reqLog)
	}

	m.Respond(err.Status, 1, err.Description, err.Message, true, w)
}

func (m WebHanlder) Respond(code, status int, message string, data any, abort bool, w webFramework.WebFramework) {
	m.RespondWithReceipt(code, status, message, data, nil, abort, w)
}

func (m WebHanlder) GetErrorsArray(message string, data any) []response.ErrorResponse {
	return response.GetErrorsArrayWithMap(message, data, m.ErrorDesc)
}

func (m WebHanlder) RespondWithReceipt(code, status int, message string, data any, printData *response.Receipt, abort bool, w webFramework.WebFramework) {
	var resp response.WsResponse
	resp.Status = status
	if code == 200 {
		resp.Description = m.MessageDesc[message]
		resp.Result = data
		resp.PrintReceipt = printData
	} else {
		resp.ErrorData = m.GetErrorsArray(message, data)
	}

	if r := w.Parser.GetLocal("reqLog"); r != nil {
		reqLog := r.(*libRequest.Request)
		reqLog.Id = w.Parser.GetHeaderValue("Request-Id")
		reqLog.BranchId = w.Parser.GetHeaderValue("Branch-Id")
	}

	err := w.Parser.SendJSONRespBody(code, resp)
	if err != nil {
		log.Println("error in SendJSONRespBody", err)
	}

	if r := w.Parser.GetLocal("reqLog"); r != nil {
		reqLog := r.(*libRequest.Request)
		if len(message) > 63 {
			reqLog.Result = message[:63]
		} else {
			reqLog.Result = message
		}

		reqLog.Outgoing = resp //string(respB)
		if message != "DUPLICATE_REQUEST" {
			err := m.RequestInterface.UpdateRequestWithContext(w.Ctx, *reqLog)
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
