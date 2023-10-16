package requestCore

import (
	"context"
	"log"

	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

type HandlerInterface[Req any, Resp any] interface {
	// returns Request Bodymode and validate header option
	Parameters() (libRequest.Type, bool)
	// runs after validating request
	Initializer(req HandlerRequest[Req, Resp]) *response.ErrorState
	// main handler runs after initialize
	Handler(req HandlerRequest[Req, Resp]) (Resp, *response.ErrorState)
	// runs after sending back response
	Finalizer(req HandlerRequest[Req, Resp])
}

type HandlerRequest[Req any, Resp any] struct {
	Title    string
	Header   *libRequest.RequestHeader
	Request  *Req
	Response Resp
	W        webFramework.WebFramework
	Args     []any
}

func BaseHandler[Req any, Resp any, Handler HandlerInterface[Req, Resp]](title string,
	core RequestCoreInterface,
	handler Handler,
	args ...any,
) any {
	log.Println("Registering: ", title)
	return func(c context.Context) {
		req := HandlerRequest[Req, Resp]{
			Title: title,
			Args:  args,
		}
		req.W = libContext.InitContext(c)
		mode, validateHeader := handler.Parameters()
		var errParse response.Err
		req.Request, req.Header, errParse = libRequest.ParseRequest[Req](req.W, mode, validateHeader)
		if errParse != nil {
			core.Responder().Error(req.W, errParse)
			return
		}

		errInit := handler.Initializer(req)
		if errInit != nil {
			core.Responder().Error(req.W, errInit)
			return
		}

		resp, err := handler.Handler(req)
		if err != nil {
			core.Responder().Error(req.W, err)
			return
		}

		core.Responder().OK(req.W, resp)
		handler.Finalizer(req)
	}
}
