package initiator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libParams"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
)

type AppEnv struct {
	Params    libParams.ParamInterface
	Interface requestCore.RequestCoreInterface
}

const DefaultRequestFields = "id,dt,incoming,action_id,national_id,branch_id,user_id,outgoing,result,events"

func GetEnv(
	name, title string,
	wsParams libParams.ParamInterface,
	requestFields string,
) *requestCore.RequestCoreModel {
	queryRunner := libQuery.Init(
		wsParams.GetDB(name).Db,
		filepath.Base(os.Args[0]),
		title,
		libQuery.Postgres,
	)
	var requestHandler libRequest.RequestInterface
	switch requestFields {
	case NoRequest:
		requestHandler = NoReq{}
	case LogRequest:
		requestHandler = libRequest.LogRequest{}
	default:
		requestHandler = libRequest.RequestModel{
			QueryInterface: queryRunner,
			InsertInDb: fmt.Sprintf(`--sql
						INSERT INTO %s.request 
						select  %s
						from 	json_populate_record(NULL::%s.request, $1::json)`, name, requestFields, name),
			UpdateInDb: fmt.Sprintf(`--sql
						UPDATE %s.request 
						set (result, outgoing, events) =
							(
								select result, outgoing, events
								from json_populate_record(NULL::%s.request, $1::json)
							) 
						where id = $2`, name, name),
			QueryInDb: fmt.Sprintf("SELECT * from %s.request where id = $1 ", name),
		}
	}
	model := requestCore.RequestCoreModel{
		QueryInterface:   queryRunner,
		RequestInterface: requestHandler,
		RespHandler: response.WebHanlder{
			ErrorDesc:   wsParams.GetConstants(name).ErrorDesc,
			MessageDesc: wsParams.GetConstants(name).MessageDesc,
		},
		ParamMap: wsParams,
	}
	return &model
}
