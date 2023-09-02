package libContext

import (
	"context"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/hmmftg/requestCore/libFiber"
	"github.com/hmmftg/requestCore/libGin"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
	"github.com/valyala/fasthttp"
)

const (
	WebFrameworkKey = libQuery.ContextKey("webFramework")
	Gin             = "gin"
	Fiber           = "fiber"
)

func InitContext(c context.Context) webFramework.WebFramework {
	w := webFramework.WebFramework{}
	switch c.(type) {
	case *gin.Context:
		w.Ctx = context.WithValue(c, WebFrameworkKey, Gin)
		w.Parser = libGin.InitContext(c)
	case *fasthttp.RequestCtx:
		w.Ctx = context.WithValue(c, WebFrameworkKey, Fiber)
		w.Parser = libFiber.InitContext(c)
	default:
		log.Fatalf("error in InitContext: %s is unknown webFramework", c.Value(WebFrameworkKey).(string))
	}
	w.Ctx = context.WithValue(w.Ctx, libQuery.ContextKey(libQuery.USER), w.Parser.GetHeaderValue("User-Id"))
	return w
}

func InitContextWithHandler(c context.Context, handler response.ResponseHandler) webFramework.WebFramework {
	return InitContext(c)
}
