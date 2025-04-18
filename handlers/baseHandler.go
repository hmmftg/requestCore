package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

type HandlerParameters struct {
	Title           string
	Body            libRequest.Type
	ValidateHeader  bool
	SaveToRequest   bool
	Path            string
	HasReceipt      bool
	RecoveryHandler func(any)
	FileResponse    bool
	LogArrays       []string
	LogTags         []string
}

type HandlerInterface[Req any, Resp any] interface {
	// returns handler title
	//   Request Bodymode
	//   and validate header option
	//   and save to request table option
	//   and url path of handler
	Parameters() HandlerParameters
	// runs after validating request
	Initializer(req HandlerRequest[Req, Resp]) error
	// main handler runs after initialize
	Handler(req HandlerRequest[Req, Resp]) (Resp, error)
	// runs after sending back response
	Finalizer(req HandlerRequest[Req, Resp])
	// handles simulation mode
	Simulation(req HandlerRequest[Req, Resp]) (Resp, error)
}

type HandlerRequest[Req any, Resp any] struct {
	Title    string
	Core     requestCore.RequestCoreInterface
	Header   *libRequest.RequestHeader
	Request  *Req
	Response Resp
	W        webFramework.WebFramework
	Args     []any
	RespSent bool
	Builder  func(status int, rawResp []byte, headers map[string]string) (*Resp, error)
}

func BaseHandler[Req any, Resp any, Handler HandlerInterface[Req, Resp]](
	core requestCore.RequestCoreInterface,
	handler Handler,
	simulation bool,
	args ...any,
) any {
	params := handler.Parameters()
	webFramework.AddServiceRegistrationLog(params.Title)
	return func(c context.Context) {
		start := time.Now()
		var w webFramework.WebFramework
		if params.SaveToRequest {
			w = libContext.InitContext(c)
		} else {
			w = libContext.InitContextNoAuditTrail(c)
		}
		libContext.AddWebLogs(w, params.Title, webFramework.HandlerLogTag)
		trx := HandlerRequest[Req, Resp]{
			Title: params.Title,
			Args:  args,
			Core:  core,
			W:     w,
		}

		defer Recovery(start, w, handler, params, trx, core)

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

			var err error
			trx.Response, err = handler.Simulation(trx)
			if err != nil {
				core.Responder().Error(trx.W, err)
				return
			}

			core.Responder().OK(trx.W, trx.Response)
		}

		var errParse error
		trx.Request, trx.Header, errParse = libRequest.ParseRequest[Req](
			trx.W,
			params.Body,
			params.ValidateHeader)
		if errParse != nil {
			core.Responder().Error(trx.W, errParse)
			return
		}
		webFramework.AddLog(w, webFramework.HandlerLogTag, slog.Any("request", trx.Request))

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
		webFramework.AddLog(w, webFramework.HandlerLogTag, slog.Any("initialize", errInit))
		if errInit != nil {
			core.Responder().Error(trx.W, errInit)
			return
		}

		var err error
		trx.Response, err = handler.Handler(trx)
		webFramework.AddLog(w, webFramework.HandlerLogTag, slog.Any("main-handler", err))
		if err != nil {
			core.Responder().Error(trx.W, err)
			return
		}
		webFramework.AddLog(w, webFramework.HandlerLogTag, slog.Any("response", trx.Response))
		if params.HasReceipt {
			receipt := trx.W.Parser.GetLocal("receipt")
			if receipt != nil {
				rc, ok := receipt.(*response.Receipt)
				if ok {
					core.Responder().OKWithReceipt(trx.W, trx.Response, rc)
					trx.RespSent = true
				} else {
					slog.Error("registered as handler with receipt, but receipt local was", slog.Any("receipt", fmt.Sprintf("%t", receipt)))
				}
			} else {
				slog.Error("registered as handler with receipt, but receipt local was absent")
			}
		}

		if params.FileResponse {
			attachment := trx.W.Parser.GetLocal("attachment")
			if attachment != nil {
				rc, ok := attachment.(*response.FileResponse)
				if ok {
					core.Responder().OKWithAttachment(trx.W, rc)
					trx.RespSent = true
				} else {
					slog.Error("registered as handler with attachment, but attachment local was", slog.Any("receipt", fmt.Sprintf("%t", attachment)))
				}
			} else {
				slog.Error("registered as handler with attachment, but attachment local was absent")
			}
		}

		if !trx.RespSent {
			core.Responder().OK(trx.W, trx.Response)
			trx.RespSent = true
		}
	}
}
