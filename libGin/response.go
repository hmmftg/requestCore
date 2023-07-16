package libGin

import (
	"fmt"
	"log"

	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"

	"github.com/gin-gonic/gin"
	"github.com/hmmftg/stacktrace"
)

func (m GinModel) HandleErrorState(err error, status int, message string, data any, ctx any) {
	c := ctx.(*gin.Context)
	log.Println(err.Error(), stacktrace.PropagateWithDepth(err, 1, ""))

	if r, ok := c.Get("reqLog"); ok {
		reqLog := r.(*libRequest.Request)
		m.RequestInterface.LogEnd("HandleError", fmt.Sprintf("desc: %s, err: %v, data: %v", message, err, data), reqLog)
	}
	switch internalError := err.(type) {
	case response.InternalError:
		m.Respond(status, 1, internalError.Desc, internalError.Message, true, c)
		return
	}
	m.Respond(status, 1, message, data, true, c)
}

func (m GinModel) ErrorState(ctx any, err *response.ErrorState) {
	c := ctx.(*gin.Context)
	log.Println(err.Error(), stacktrace.PropagateWithDepth(err, 1, ""))

	if r, ok := c.Get("reqLog"); ok {
		reqLog := r.(*libRequest.Request)
		m.RequestInterface.LogEnd("HandleError", fmt.Sprintf("desc: %s, err: %v, data: %v", err.Description, err, err.Message), reqLog)
	}

	m.Respond(err.Status, 1, err.Description, err.Message, true, c)
}

func (m GinModel) Respond(code, status int, message string, data any, abort bool, ctx any) {
	m.RespondWithReceipt(code, status, message, data, response.Receipt{}, abort, ctx)
}

func (m GinModel) RespondWithReceipt(code, status int, message string, data any, printData response.Receipt, abort bool, ctx any) {
	c := ctx.(*gin.Context)
	var resp response.WsResponse
	resp.Status = status
	if code == 200 {
		resp.Description = m.MessageDesc[message]
		resp.Result = data
		resp.PrintReceipt = &printData
	} else {
		resp.ErrorData = m.GetErrorsArray(message, data)
	}

	if r, ok := c.Get("reqLog"); ok {
		reqLog := r.(*libRequest.Request)
		c.Header("Request-Id", reqLog.Id)
		c.Header("Branch-Id", reqLog.BranchId)
	}

	c.JSON(code, resp)

	if r, ok := c.Get("reqLog"); ok {
		reqLog := r.(*libRequest.Request)
		if len(message) > 63 {
			reqLog.Result = message[:63]
		} else {
			reqLog.Result = message
		}

		reqLog.Outgoing = resp //string(respB)
		if message != "DUPLICATE_REQUEST" {
			m.RequestInterface.UpdateRequest(*reqLog)
		}
	}
	if abort {
		c.Abort()
	} else {
		c.Next()
	}
}

func Respond(code, status int, message string, data any, abort bool, ctx any) {
	c := ctx.(*gin.Context)
	var resp response.WsResponse
	resp.Status = status
	if code == 200 {
		resp.Description = message
		resp.Result = data
	} else {
		resp.ErrorData = response.GetErrorsArray(message, data)
	}

	c.JSON(code, resp)
	if abort {
		c.Abort()
	} else {
		c.Next()
	}
}
