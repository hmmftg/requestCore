package libCallApi

import (
	"net/http"

	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

// TypeList is an interface for retrieving a typed element by index.
type TypeList interface {
	GetType(int) any
}

// MultiCall executes a sequence of calls, stopping early on non-OK status.
func MultiCall(w webFramework.WebFramework, paramList []CallParam, _ CallAPIInterface) []CallResult[response.WsRemoteResponse] {
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
