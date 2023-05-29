package libFiber

import (
	"fmt"
	"net/http"

	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"

	"github.com/gofiber/fiber/v2"
	"github.com/hmmftg/stacktrace"
)

func (m FiberModel) Respond(code, status int, message string, data any, abort bool, ctx any) {
	m.RespondWithReceipt(code, status, message, data, response.Receipt{}, abort, ctx)
}

func (m FiberModel) RespondWithReceipt(code, status int, message string, data any, printData response.Receipt, abort bool, ctx any) {
	c := ctx.(*fiber.Ctx)
	var resp response.WsResponse
	resp.Status = status
	if code == 200 {
		resp.Description = m.MessageDesc[message]
		resp.Result = data
		resp.PrintReceipt = printData
	} else {
		resp.ErrorData = m.GetErrorsArray(message, data)
	}

	c.JSON(resp)

	if r := c.Locals("reqLog"); r != nil {
		reqLog := r.(*libRequest.Request)
		reqLog.Result = message
		//respB, _ := json.Marshal(resp)
		reqLog.Outgoing = resp //string(respB)
		if message != "DUPLICATE_REQUEST" {
			m.RequestInterface.UpdateRequest(*reqLog)
		}
	}
	c.Status(code)
	if abort {
		c.SendStatus(code)
	} else {
		c.Next()
	}
}

func (m FiberModel) HandleErrorState(err error, status int, message string, data any, ctx any) {
	c := ctx.(*fiber.Ctx)
	fmt.Println(err.Error(), stacktrace.PropagateWithDepth(err, 1, message, data))
	m.Respond(status, 1, message, data, true, c)
}

func Respond(code, status int, message string, data any, abort bool, ctx any) {
	c := ctx.(*fiber.Ctx)
	var resp response.WsResponse
	resp.Status = status
	if code == 200 {
		resp.Description = message
		resp.Result = data
	} else {
		resp.ErrorData = response.GetErrorsArray(message, data)
	}

	c.JSON(resp)
	c.Status(code)
	if abort {
		c.SendStatus(code)
	} else {
		c.Next()
	}
}

func ErrorHandler(path, title string, respondHandler func(int, int, string, any, bool, any)) fiber.ErrorHandler {
	fmt.Println("ErrorHandler: ", path, title)
	return func(c *fiber.Ctx, err error) error {
		fmt.Println(path, title, "ErrorHandler", err)
		switch err := err.(type) {
		case *fiber.Error:
			switch err.Code {
			case 404:
				respondHandler(http.StatusNotFound, 1, "PAGE_NOT_FOUND", err, true, c)
				return nil
			}
			fmt.Println("Fiber Error", err.Code, err.Message)
			respondHandler(http.StatusInternalServerError, 1, "INTERNAL_ERROR", err, true, c)
		default:
			if c.Locals("LastError") != nil {
				fmt.Println("LocalError", err)
				respondHandler(http.StatusInternalServerError, 1, c.Locals("LastError").(string), nil, true, c)
				return nil
			}
			fmt.Println("Unknown", err)
			respondHandler(http.StatusInternalServerError, 1, "INTERNAL_ERROR", nil, true, c)
			return nil
		}
		fmt.Println("Unknown", err)
		respondHandler(http.StatusInternalServerError, 1, "INTERNAL_ERROR", nil, true, c)
		return nil
	}
}
