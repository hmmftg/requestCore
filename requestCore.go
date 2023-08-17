package requestCore

import (
	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libCrypto"
	"github.com/hmmftg/requestCore/libDictionary"
	"github.com/hmmftg/requestCore/libParams"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
)

func (m RequestCoreModel) GetDB() libQuery.QueryRunnerInterface {
	return m.QueryInterface
}

func (m RequestCoreModel) RequestTools() libRequest.RequestInterface {
	return m.RequestInterface
}

func (m RequestCoreModel) Consumer() libCallApi.CallApiInterface {
	return m.RemoteApiInterface
}

func (m RequestCoreModel) Responder() response.ResponseHandler {
	return m.RespHandler
}

func (m RequestCoreModel) Dictionary() libDictionary.DictionaryInterface {
	return m.Dict
}

func (m RequestCoreModel) Params() libParams.ParamsInterface {
	return m.ParamMap
}

func (m RequestCoreModel) Sm() libCrypto.Sm {
	return m.CryptoSm
}
