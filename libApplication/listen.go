package initiator

import (
	"fmt"
	"log"
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/hmmftg/requestCore/libParams"
	"github.com/hmmftg/requestCore/webFramework"
)

// Listen starts the Gin engine on the configured HTTP or HTTPS port.
func Listen(netParams *libParams.NetworkParams, app *gin.Engine) {
	listenLog := ""
	if len(netParams.TLSPort) > 0 {
		listenLog = fmt.Sprintf("About to tls listen on %s", netParams.TLSPort)
		webFramework.AddStartUpLog(slog.String("listen", listenLog))
		webFramework.CollectStartUpLogs()
		errHTTP := app.RunTLS(":"+netParams.TLSPort, netParams.TLSCert, netParams.TLSKey)
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
