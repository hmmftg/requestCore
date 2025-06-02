package ginsplunk

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hmmftg/requestCore/libLogger"
	"github.com/hmmftg/requestCore/libLogger/splunk"
	"github.com/hmmftg/requestCore/libParams"
)

func ConfigGinSplunk(wsParams libParams.ParamInterface) gin.LoggerConfig {
	logger, err := splunk.CheckIfSplunkIsWorking(wsParams.GetLogging().Splunk)
	if err != nil {
		return libLogger.StdoutLogger{}.Config(wsParams)
	}

	// omit all logging formats and just add short filename
	log.SetFlags(log.Lshortfile)
	log.SetOutput(logger)
	log.Printf("Logger Configured %T", logger)
	return gin.LoggerConfig{
		Output:    logger,
		SkipPaths: wsParams.GetLogging().SkipPaths,
		Formatter: func(param gin.LogFormatterParams) string {
			if param.Latency > time.Minute {
				param.Latency = param.Latency.Truncate(time.Second)
			}
			lg := splunk.LogSplunk{
				Logger:     "gin",
				Severity:   "INFO",
				Header:     wsParams.GetLogging().LogHeader,
				Status:     param.StatusCode,
				Client:     param.ClientIP,
				Latency:    param.Latency.String(),
				LatencyInt: param.Latency.Microseconds(),
				Method:     param.Method,
				Path:       param.Path,
				Error:      param.ErrorMessage,
			}
			b, err := json.Marshal(lg)
			if err != nil {
				fmt.Println("unable to marshal log:", err)
			}
			return string(b)
		},
	}
}
