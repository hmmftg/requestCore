package logger

import (
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hmmftg/requestCore/libRequest"

	//cSpell: ignore natefinch
	"gopkg.in/natefinch/lumberjack.v2"
)

func ConfigGinLogger(params libRequest.LoggerInterface) gin.LoggerConfig {
	logger := lumberjack.Logger{
		Filename: params.GetLogPath(),
		MaxSize:  params.GetLogSize(), // megabytes
		//MaxBackups: 3, keep all
		//MaxAge:     28,   //days, keep all
		Compress: params.GetLogCompress(), // disabled by default
	}
	log.SetOutput(&logger)
	return gin.LoggerConfig{
		Output: &logger,
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
			return fmt.Sprintf("[GIN] %v |%s %3d %s| %13v | %15s |%s %-7s %s %#v\n%s",
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
