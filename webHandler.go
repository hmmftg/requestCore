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
	var resp response.WsResponse
	resp.Status = status
	if code == 200 {
		resp.Description = m.MessageDesc[message]
		resp.Result = data
		resp.PrintReceipt = printData
	} else {
		resp.ErrorData = m.GetErrorsArray(message, data)
	}

	err := w.Parser.SendJSONRespBody(code, resp)
	if err != nil {
		log.Println("error in SendJSONRespBody", err)
	}

	if r := w.Parser.GetLocal("reqLog"); r != nil {
		reqLog := r.(libRequest.RequestPtr)
		reqLog.Id = w.Parser.GetHeaderValue("Request-Id")
		reqLog.BranchId = w.Parser.GetHeaderValue("Branch-Id")
		if len(message) > 63 {
			reqLog.Result = message[:63]
		} else {
			reqLog.Result = message
		}

		reqLog.Outgoing = resp //string(respB)
		if message != "DUPLICATE_REQUEST" {
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

func (m WebHanlder) RespondWithAttachment(code, status int, message string, file *response.FileResponse, abort bool, w webFramework.WebFramework) {
	var resp response.WsResponse
	resp.Status = status
	if code == 200 {
		resp.Description = m.MessageDesc[message]

		w.Parser.FileAttachment(file.Path, file.FileName)
	} else {
		resp.ErrorData = m.GetErrorsArray(message, nil)
	}

	if status == 200 {
		resp.Description = m.MessageDesc[message]

		w.Parser.FileAttachment(file.Path, file.FileName)
	} else {
		resp.ErrorData = m.GetErrorsArray(message, nil)
	}

	if r := w.Parser.GetLocal("reqLog"); r != nil {
		reqLog := r.(libRequest.RequestPtr)
		reqLog.Id = w.Parser.GetHeaderValue("Request-Id")
		reqLog.BranchId = w.Parser.GetHeaderValue("Branch-Id")
		if len(message) > 63 {
			reqLog.Result = message[:63]
		} else {
			reqLog.Result = message
		}

		reqLog.Outgoing = resp //string(respB)
		if message != "DUPLICATE_REQUEST" {
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
