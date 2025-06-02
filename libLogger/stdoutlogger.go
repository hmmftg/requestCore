package libLogger

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hmmftg/requestCore/libParams"
)

type StdoutLogger struct {
}

func (logger StdoutLogger) Write(p []byte) (n int, err error) {
	return os.Stdout.WriteString(string(p))
}

func (logger StdoutLogger) Config(wsParams libParams.ParamInterface) gin.LoggerConfig {
	log.SetOutput(logger)
	log.Printf("Logger Configured %T \n", logger)
	return gin.LoggerConfig{
		Output:    logger,
		SkipPaths: wsParams.GetLogging().SkipPaths,
		Formatter: func(param gin.LogFormatterParams) string {
			var statusColor, methodColor, resetColor string
			if param.IsOutputColor() {
				statusColor = param.StatusCodeColor()
				methodColor = param.MethodColor()
				resetColor = param.ResetColor()
			}

			if param.Latency > time.Minute {
				param.Latency = param.Latency.Truncate(time.Second)
			}
			return fmt.Sprintf("[%s] %v |%s %3d %s| %13v | %15s |%s %-7s %s %#v\n%s",
				wsParams.GetLogging().LogHeader,
				param.TimeStamp.Format("2006/01/02 - 15:04:05"),
				statusColor, param.StatusCode, resetColor,
				param.Latency,
				param.ClientIP,
				methodColor, param.Method, resetColor,
				param.Path,
				param.ErrorMessage,
			)
		},
	}
}
