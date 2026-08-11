package requestCore

import (
	"github.com/hmmftg/requestCore/libParams"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libQuery/liborm"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
)

// RequestCoreModel is the central model holding query, ORM, request, response, and parameter interfaces.
//
//revive:disable-next-line:exported
type RequestCoreModel struct {
	RequestInterface libRequest.RequestInterface
	QueryInterface   libQuery.QueryRunnerInterface
	OrmInterface     liborm.OrmInterface
	RespHandler      response.ResponseHandler
	ParamMap         libParams.ParamInterface
}

// RequestCoreInterface defines the accessor methods for the core request processing model.
//
//revive:disable-next-line:exported
type RequestCoreInterface interface {
	GetDB() libQuery.QueryRunnerInterface
	ORM() liborm.OrmInterface
	RequestTools() libRequest.RequestInterface
	Responder() response.ResponseHandler
	Params() libParams.ParamInterface
}
