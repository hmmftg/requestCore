package gininitiator

import (
	"encoding/json"
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/hmmftg/requestCore/libGin/logger"
	"github.com/hmmftg/requestCore/libGin/logger/ginsplunk"
	"github.com/hmmftg/requestCore/libParams"
)

var HandlePanics gin.RecoveryFunc = func(c *gin.Context, err any) {
	log.Printf("panic happened!!! err: %+v", err)
	c.Abort()
}

func InitGin(basePath string, wsParams libParams.ParamInterface, corsConfig *cors.Config) (*gin.Engine, *gin.RouterGroup) {
	app := gin.New()
	logParams := wsParams.GetLogging()

	if logParams.UseSlog {
		InitSlogGin(wsParams, app)
	} else {
		var logging gin.LoggerConfig
		if logParams.Splunk != nil {
			logging = ginsplunk.ConfigGinSplunk(wsParams)
			app.Use(gin.RecoveryWithWriter(logging.Output, HandlePanics))
		} else {
			if len(logParams.LogPath) == 0 {
				logging = gin.LoggerConfig{
					SkipPaths: logging.SkipPaths,
				}
				bParams, err := json.MarshalIndent(wsParams, "", "  ")
				if err != nil {
					log.Fatal(err.Error())
				}
				log.Println(string(bParams))
				app.Use(gin.CustomRecovery(HandlePanics))
			} else {
				logging = logger.ConfigGinLogger(wsParams.GetLogging())
				app.Use(gin.RecoveryWithWriter(logging.Output, HandlePanics))
			}
		}
		log.Printf("gin logger: %+v", logging)
		app.Use(gin.LoggerWithConfig(logging))
	}

	app.Use(cors.New(*corsConfig))

	g := app.Group(basePath)
	return app, g
}
