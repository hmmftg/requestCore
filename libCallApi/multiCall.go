package libCallApi

import (
	"net/http"

	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

type TypeList interface {
	GetType(int) any
}

func MultiCall(w webFramework.WebFramework, paramList []CallParam, core CallApiInterface) []CallResult[response.WsRemoteResponse] {
	resultList := make([]CallResult[response.WsRemoteResponse], 0)
	for i := 0; i < len(paramList); i++ {
		resp := Call[response.WsRemoteResponse](w, paramList[i])
		resultList = append(resultList, resp)
		if resp.Status.Status != http.StatusOK {
			return resultList
		}
	}
	return resultList
}
