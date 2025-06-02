package gininitiator

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hmmftg/requestCore/libLogger/splunk"
	"github.com/hmmftg/requestCore/libParams"
	"github.com/hmmftg/requestCore/webFramework"
	slogformatter "github.com/samber/slog-formatter"
	sloggin "github.com/samber/slog-gin"
)

func InitSlogGin(wsParams libParams.ParamInterface, app *gin.Engine) {
	var logWriter io.Writer
	splunkLogger, err := splunk.CheckIfSplunkIsWorking(wsParams.GetLogging().Splunk)
	if err != nil {
		logWriter = os.Stdout
	} else {
		// omit all logging formats and just add short filename
		log.SetFlags(log.Lshortfile)
		log.SetOutput(splunkLogger)
		logWriter = splunkLogger
	}
	webFramework.AddStartUpLog(slog.String("Logger", fmt.Sprintf("Configured as %T", logWriter)))

	// Create a slog logger, which:
	//   - Logs to stdout.
	logger := slog.New(
		slogformatter.NewFormatterHandler(
			slogformatter.TimeFormatter(time.DateTime, time.Local),
		)(
			slog.NewJSONHandler(logWriter, &slog.HandlerOptions{
				/*ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
					if a.Key == "body" {
						if strings.Contains(a.Value.String(), "access_token") {
							var resp auth.LoginResponse
							err := json.Unmarshal([]byte(a.Value.String()), &resp)
							if err == nil {
								return slog.Attr{Key: a.Key, Value: slog.AnyValue(resp)}
							}
						}
						if strings.Contains(a.Value.String(), "track2Data") {
							var resp handlers.WsResponse[postcartino.CardPostResp]
							err := json.Unmarshal([]byte(a.Value.String()), &resp)
							if err == nil {
								return slog.Attr{Key: a.Key, Value: slog.AnyValue(resp.Result)}
							}
						}
					}
					return a
				},*/
			}),
		),
	) //.WithGroup("http")
	slog.SetDefault(logger)

	config := sloggin.Config{
		WithRequestBody:    true,
		WithResponseBody:   true,
		WithRequestHeader:  true,
		WithResponseHeader: true,
		Filters: []sloggin.Filter{
			sloggin.IgnorePath(wsParams.GetLogging().SkipPaths...),
		},
	}

	// Add the sloggin middleware to all routes.
	// The middleware will log all requests attributes.
	app.Use(sloggin.NewWithConfig(logger, config))
}
