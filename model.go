package requestCore

import (
	"github.com/hmmftg/requestCore/libParams"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libQuery/liborm"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
)

type RequestCoreModel struct {
	RequestInterface libRequest.RequestInterface
	QueryInterface   libQuery.QueryRunnerInterface
	OrmInterface     liborm.OrmInterface
	RespHandler      response.ResponseHandler
	ParamMap         libParams.ParamInterface
}

type RequestCoreInterface interface {
	GetDB() libQuery.QueryRunnerInterface
	ORM() liborm.OrmInterface
	RequestTools() libRequest.RequestInterface
	Responder() response.ResponseHandler
	Params() libParams.ParamInterface
}
