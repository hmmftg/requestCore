package logger

import (
	"log"

	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/hmmftg/requestCore/libRequest"
	"gopkg.in/natefinch/lumberjack.v2"
)

func ConfigFiberLogger(params libRequest.LoggerInterface) logger.Config {
	logWriter := lumberjack.Logger{
		Filename: params.GetLogPath(),
		MaxSize:  params.GetLogSize(), // megabytes
		//MaxBackups: 3, keep all
		//MaxAge:     28,   //days, keep all
		Compress: params.GetLogCompress(), // disabled by default
	}
	log.SetOutput(&logWriter)
	return logger.Config{
		Output: &logWriter,
		Format: "[${time}] ${ip} ${status} - ${latency} ${method} ${route} ${path} ${error}\n",
	}
}
