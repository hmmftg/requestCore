package logger

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
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
	go func() {
		for {
			t := time.Now()
			t = t.Truncate(time.Hour * 24)

			<-time.After(time.Duration(t.Hour()) * 24)
			logWriter.Rotate()
		}
	}()
	log.SetOutput(&logWriter)
	return logger.Config{
		Output:     &logWriter,
		Format:     "[${time}] ${ip} ${status} - ${latency} ${method} ${route} ${path} ${error}\n",
		TimeFormat: "15:04:05",
		TimeZone:   "local",
		Next: func(c *fiber.Ctx) bool {
			skipPaths := params.GetSkipPaths()
			for _, skipPath := range skipPaths {
				if skipPath == c.Path() {
					return true
				}
			}
			return false
		},
	}
}
