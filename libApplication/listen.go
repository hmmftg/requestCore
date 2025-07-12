package initiator

import (
	"fmt"
	"log"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/hmmftg/requestCore/libParams"
	"github.com/hmmftg/requestCore/webFramework"
)

func Listen(netParams *libParams.NetworkParams, app *gin.Engine) {
	listenLog := ""
	if len(netParams.TlsPort) > 0 {
		listenLog = fmt.Sprintf("About to tls listen on %s", netParams.TlsPort)
		webFramework.AddStartUpLog(slog.String("listen", listenLog))
		webFramework.CollectStartUpLogs()
		errHTTP := app.RunTLS(":"+netParams.TlsPort, netParams.TlsCert, netParams.TlsKey)
		if errHTTP != nil {
			log.Fatal("Web server (HTTPS): ", errHTTP)
		}
	} else {
		listenLog = fmt.Sprintf("About to listen on %s", netParams.Port)
		webFramework.AddStartUpLog(slog.String("listen", listenLog))
		webFramework.CollectStartUpLogs()
		errHTTP := app.Run(":" + netParams.Port)
		if errHTTP != nil {
			log.Fatal("Web server (HTTP): ", errHTTP)
		}
	}
}
