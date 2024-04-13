package testingtools

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
)

func InitSimulationHandler[Req any](c context.Context, core requestCore.RequestCoreInterface) (*Req, error) {
	w := libContext.InitContext(c)
	code, desc, arrayErr, request, _, err := libRequest.GetRequest[Req](w, false)
	if err != nil {
		core.Responder().HandleErrorState(err, code, desc, arrayErr, w)
		return nil, err
	}
	return &request, nil
}

func GetSingleSimulationHandler[Req any](core requestCore.RequestCoreInterface) gin.HandlerFunc {
	return func(c *gin.Context) {
		w := libContext.InitContext(c)
		request, err := InitSimulationHandler[Req](c, core)
		if err != nil {
			return
		}

		respData := response.RespData{
			Code:    http.StatusOK,
			Status:  0,
			Message: "OK",
			Type:    response.Json,
			JSON:    request,
		}

		core.Responder().Respond(respData, false, w)
	}
}

func GetAllSimulationHandler[Req any](core requestCore.RequestCoreInterface) gin.HandlerFunc {
	return func(c *gin.Context) {
		w := libContext.InitContext(c)
		request, err := InitSimulationHandler[Req](c, core)
		if err != nil {
			return
		}

		respData := response.RespData{
			Code:    http.StatusOK,
			Status:  0,
			Message: "OK",
			Type:    response.Json,
			JSON:    []Req{*request},
		}

		core.Responder().Respond(respData, false, w)
	}
}
