package requestCore

import (
	"github.com/hmmftg/requestCore/libParams"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libQuery/liborm"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
)

func (m RequestCoreModel) GetDB() libQuery.QueryRunnerInterface {
	return m.QueryInterface
}

func (m RequestCoreModel) ORM() liborm.OrmInterface {
	return m.OrmInterface
}

func (m RequestCoreModel) RequestTools() libRequest.RequestInterface {
	return m.RequestInterface
}

func (m RequestCoreModel) Responder() response.ResponseHandler {
	return m.RespHandler
}

func (m RequestCoreModel) Params() libParams.ParamInterface {
	return m.ParamMap
}
