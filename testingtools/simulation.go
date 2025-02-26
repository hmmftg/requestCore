package testingtools

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libRequest"
)

func InitSimulationHandler[Req any](c context.Context, core requestCore.RequestCoreInterface) (*Req, error) {
	w := libContext.InitContext(c)
	result, err := libRequest.Req[Req, libRequest.RequestHeader](libRequest.ParseParams{W: w, ValidateHeader: false})
	if err != nil {
		core.Responder().Error(w, err)
		return nil, err
	}
	return &result.Request, nil
}

func GetSingleSimulationHandler[Req any](core requestCore.RequestCoreInterface) gin.HandlerFunc {
	return func(c *gin.Context) {
		w := libContext.InitContext(c)
		request, err := InitSimulationHandler[Req](c, core)
		if err != nil {
			return
		}
		core.Responder().OK(w, request)
	}
}

func GetAllSimulationHandler[Req any](core requestCore.RequestCoreInterface) gin.HandlerFunc {
	return func(c *gin.Context) {
		w := libContext.InitContext(c)
		request, err := InitSimulationHandler[Req](c, core)
		if err != nil {
			return
		}
		core.Responder().OK(w, []Req{*request})
	}
}
