package libContext

import (
	"context"
	"log"

	"github.com/hmmftg/requestCore/libFiber"
	"github.com/hmmftg/requestCore/libGin"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

type ContextKey string

const (
	WebFrameworkKey = ContextKey("webFramework")
	Gin             = "gin"
	Fiber           = "fiber"
	USER            = ContextKey("USER")
)

func InitContext(c context.Context) webFramework.WebFramework {
	w := webFramework.WebFramework{}
	switch c.Value(WebFrameworkKey) {
	case Fiber:
		w.Parser = libFiber.InitContext(c)
	case Gin:
		w.Parser = libGin.InitContext(c)
	default:
		log.Fatalf("error in InitContext: %s is unknown webFramework", c.Value(WebFrameworkKey).(string))
	}
	w.Ctx = context.WithValue(c, USER, w.Parser.GetHeaderValue("User-Id"))
	return w
}

func InitContextWithHandler(c context.Context, handler response.ResponseHandler) webFramework.WebFramework {
	return InitContext(c)
}
