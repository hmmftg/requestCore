package handlers

import (
	"context"
	"log"
	"net/http"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

type HandlerInterface[Req any, Resp any] interface {
	// returns handler title
	//   Request Bodymode
	//   and validate header option
	//   and save to request table option
	//   and url path of handler
	Parameters() (string, libRequest.Type, bool, bool, string)
	// runs after validating request
	Initializer(req HandlerRequest[Req, Resp]) response.ErrorState
	// main handler runs after initialize
	Handler(req HandlerRequest[Req, Resp]) (Resp, response.ErrorState)
	// runs after sending back response
	Finalizer(req HandlerRequest[Req, Resp])
}

type HandlerRequest[Req any, Resp any] struct {
	Title    string
	Core     requestCore.RequestCoreInterface
	Header   *libRequest.RequestHeader
	Request  *Req
	Response Resp
	W        webFramework.WebFramework
	Args     []any
}

func BaseHandler[Req any, Resp any, Handler HandlerInterface[Req, Resp]](
	core requestCore.RequestCoreInterface,
	handler Handler,
	args ...any,
) any {
	title, mode, validateHeader, saveInRequestTable, path := handler.Parameters()
	log.Println("Registering: ", title)
	return func(c context.Context) {
		w := libContext.InitContext(c)
		defer func() {
			if r := recover(); r != nil {
				core.Responder().Error(w,
					response.Error(
						http.StatusInternalServerError,
						response.SYSTEM_FAULT,
						response.SYSTEM_FAULT_DESC,
						libError.Join(r.(error), "error in %s", title),
					))
				panic(r)
			}
		}()
		trx := HandlerRequest[Req, Resp]{
			Title: title,
			Args:  args,
			Core:  core,
			W:     w,
		}
		var errParse response.ErrorState
		trx.Request, trx.Header, errParse = libRequest.ParseRequest[Req](trx.W, mode, validateHeader)
		if errParse != nil {
			core.Responder().Error(trx.W, errParse)
			return
		}

		if saveInRequestTable {
			errInitRequest := core.RequestTools().InitRequest(trx.W, title, path)
			if errInitRequest != nil {
				core.Responder().Error(trx.W, errInitRequest)
				return
			}
		}

		errInit := handler.Initializer(trx)
		if errInit != nil {
			core.Responder().Error(trx.W, errInit)
			return
		}

		var err response.ErrorState
		trx.Response, err = handler.Handler(trx)
		if err != nil {
			core.Responder().Error(trx.W, err)
			return
		}

		core.Responder().OK(trx.W, trx.Response)
		handler.Finalizer(trx)
	}
}
