package initiator

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Depado/ginprom"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libApplication/metrics"
	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libContext"
	gininitiator "github.com/hmmftg/requestCore/libGin/initiator"
	"github.com/hmmftg/requestCore/libParams"
	"github.com/hmmftg/requestCore/swagger"
	"github.com/hmmftg/requestCore/webFramework"
	"github.com/swaggo/swag/v2"

	_ "github.com/lib/pq"
	_ "github.com/sijms/go-ora/v2"
)

type Application[T any] interface {
	AddRoutes(
		model *requestCore.RequestCoreModel,
		wsParams *libParams.ApplicationParams[T],
		roleMap map[string]string,
		rg *gin.RouterGroup,
	)
	Title() string
	Name() string
	Version() string
	BasePath() string
	HasSwagger() bool
	SwaggerSpec() *swag.Spec
	RequestFields() string
	InitGinApp(*gin.Engine)
	InitParams(wsParams *libParams.ApplicationParams[T])
	GetDbList() []string
	GetKeys() [][]byte
	GetCorsConfig() *cors.Config
}

const NoRequest = "no_req"
const LogRequest = "log_req"

type App[T any] struct {
	Instance Application[T]
	Params   *libParams.ApplicationParams[T]
	Engine   *gin.Engine
	Model    requestCore.RequestCoreInterface
}

//nolint:lll
func InitializeApp[T any](app Application[T]) *App[T] {
	var err error
	log.Printf("%s Version:\t %s\n", app.Title(), app.Version())
	paramName := os.Getenv("PARAM_NAME")
	if len(paramName) == 0 {
		paramName = "param.yaml"
	}
	paramFile := flag.String("p", paramName, "Application Params")

	flag.Parse()

	wsParams, err := libParams.ParsePrams[T](*paramFile, app.GetKeys())
	if err != nil {
		log.Fatal("InitializeApp: ParsePrams=>", err)
	}

	InitDataBases(wsParams, app.GetDbList())

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	engine, root := gininitiator.InitGin(app.BasePath(), wsParams, app.GetCorsConfig())

	safeTitle := strings.ReplaceAll(app.Title(), ".", "_")
	safeTitle = strings.ReplaceAll(safeTitle, " ", "_")
	safeName := strings.ReplaceAll(app.Name(), ".", "_")
	safeName = strings.ReplaceAll(strings.ReplaceAll(safeName, " ", "_"), "-", "_")

	app.InitParams(wsParams)

	model := GetEnv(safeName, app.Title(), wsParams, app.RequestFields())

	wsParams.Metrics = ginprom.New(
		ginprom.Engine(engine),
		ginprom.Namespace(safeTitle),
		ginprom.Subsystem(safeName),
		// ginprom.Path("/metrics"),
	)
	safeVersion := strings.ReplaceAll(app.Version(), ".", "_")
	wsParams.Metrics.AddCustomGauge("version", "Version.", []string{"Version"})
	err = wsParams.Metrics.SetGaugeValue("version", []string{safeVersion}, 1)
	if err != nil {
		log.Fatal("SetGaugeValue: version", safeVersion, err)
	}
	wsParams.Metrics.AddCustomCounter("up_time", fmt.Sprintf("%s Up-Time.", app.Name()), nil)
	go metrics.RecordUptime("up_time", wsParams.Metrics)
	engine.Use(wsParams.Metrics.Instrument())

	app.InitGinApp(engine)

	for id := range wsParams.RemoteApis {
		api := wsParams.RemoteApis[id]
		cache, lock := libCallApi.InitTokenCache()
		api.TokenCache = cache
		api.TokenCacheLock = lock
		wsParams.RemoteApis[id] = api
	}

	app.AddRoutes(
		model,
		wsParams,
		nil,
		root,
	)
	if wsParams.Logging.UseSlog {
		webFramework.CollectServiceRegistrationLogs()
	}

	if app.HasSwagger() {
		engine.GET(fmt.Sprintf("%s/%s/swagger/*any", app.BasePath(), app.Name()),
			swagger.GinHandler(app.BasePath(), app.Name(), app.SwaggerSpec()),
		)
		engine.GET(fmt.Sprintf("%s/%s/swagger", app.BasePath(), app.Name()), func(c *gin.Context) {
			start := time.Now()
			finalize := libContext.AddWebHandlerLogs(c, "Redirect", "redirect-handler")
			defer finalize(start, http.StatusTemporaryRedirect)
			c.Redirect(http.StatusTemporaryRedirect, "swagger/index.html")
		})
	}

	return &App[T]{app, wsParams, engine, model}
}

func StartApp[T any](app App[T]) {
	Listen(app.Params.GetNetwork(app.Instance.Name()), app.Engine)
}
