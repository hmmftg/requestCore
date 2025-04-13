package requestCore

import (
	"github.com/hmmftg/requestCore/libParams"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
)

type RequestCoreModel struct {
	RequestInterface libRequest.RequestInterface
	QueryInterface   libQuery.QueryRunnerInterface
	RespHandler      response.ResponseHandler
	ParamMap         libParams.ParamInterface
}

type RequestCoreInterface interface {
	GetDB() libQuery.QueryRunnerInterface
	RequestTools() libRequest.RequestInterface
	Responder() response.ResponseHandler
	Params() libParams.ParamInterface
}
