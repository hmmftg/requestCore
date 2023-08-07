package requestCore

import (
	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libDictionary"
	"github.com/hmmftg/requestCore/libParams"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
)

type RequestCoreModel struct {
	RequestInterface   libRequest.RequestInterface
	QueryInterface     libQuery.QueryRunnerInterface
	RemoteApiInterface libCallApi.CallApiInterface
	RespHandler        response.ResponseHandler
	Dict               libDictionary.DictionaryInterface
	ParamMap           libParams.ParamsInterface
}

type RequestCoreInterface interface {
	GetDB() libQuery.QueryRunnerInterface
	RequestTools() libRequest.RequestInterface
	Consumer() libCallApi.CallApiInterface
	Responder() response.ResponseHandler
	Dictionary() libDictionary.DictionaryInterface
	Params() libParams.ParamsInterface
}
