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

type HandlerParameters struct {
	Title          string
	Body           libRequest.Type
	ValidateHeader bool
	SaveToRequest  bool
	Path           string
	HasReceipt     bool
}

type HandlerInterface[Req any, Resp any] interface {
	// returns handler title
	//   Request Bodymode
	//   and validate header option
	//   and save to request table option
	//   and url path of handler
	Parameters() HandlerParameters
	// runs after validating request
	Initializer(req HandlerRequest[Req, Resp]) response.ErrorState
	// main handler runs after initialize
	Handler(req HandlerRequest[Req, Resp]) (Resp, response.ErrorState)
	// runs after sending back response
	Finalizer(req HandlerRequest[Req, Resp])
	// handles simulation mode
	Simulation(req HandlerRequest[Req, Resp]) (Resp, response.ErrorState)
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
	simulation bool,
	args ...any,
) any {
	params := handler.Parameters()
	log.Println("Registering: ", params.Title)
	return func(c context.Context) {
		var w webFramework.WebFramework
		if params.SaveToRequest {
			w = libContext.InitContext(c)
		} else {
			w = libContext.InitContextNoAuditTrail(c)
		}
		defer func() {
			if r := recover(); r != nil {
				core.Responder().Error(w,
					response.Error(
						http.StatusInternalServerError,
						response.SYSTEM_FAULT,
						response.SYSTEM_FAULT_DESC,
						libError.Join(r.(error), "error in %s", params.Title),
					))
				panic(r)
			}
		}()
		trx := HandlerRequest[Req, Resp]{
			Title: params.Title,
			Args:  args,
			Core:  core,
			W:     w,
		}

		if simulation {
			resp, header, errParse := libRequest.ParseRequest[Resp](
				trx.W,
				params.Body,
				params.ValidateHeader,
			)
			if errParse != nil {
				core.Responder().Error(trx.W, errParse)
				return
			}

			trx.Response = *resp
			trx.Header = header

			var err response.ErrorState
			trx.Response, err = handler.Simulation(trx)
			if err != nil {
				core.Responder().Error(trx.W, err)
				return
			}

			core.Responder().OK(trx.W, trx.Response)
		}

		var errParse response.ErrorState
		trx.Request, trx.Header, errParse = libRequest.ParseRequest[Req](
			trx.W,
			params.Body,
			params.ValidateHeader)
		if errParse != nil {
			core.Responder().Error(trx.W, errParse)
			return
		}

		if params.SaveToRequest {
			errInitRequest := core.RequestTools().InitRequest(trx.W, params.Title, params.Path)
			if errInitRequest != nil {
				core.Responder().Error(trx.W, errInitRequest)
				return
			}
		} else {
			w.Parser.SetLocal("reqLog", nil)
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

		respSent := false
		if params.HasReceipt {
			receipt := trx.W.Parser.GetLocal("receipt")
			if receipt != nil {
				rc, ok := receipt.(*response.Receipt)
				if ok {
					core.Responder().OKWithReceipt(trx.W, trx.Response, rc)
					respSent = true
				} else {
					log.Printf("registered as handler with receipt, but receipt local was: %t\n", receipt)
				}
			} else {
				log.Println("registered as handler with receipt, but receipt local was abset")
			}
		}
		if !respSent {
			core.Responder().OK(trx.W, trx.Response)
		}

		handler.Finalizer(trx)
	}
}
