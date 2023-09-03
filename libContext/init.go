package libContext

import (
	"context"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
	"github.com/hmmftg/requestCore/libFiber"
	"github.com/hmmftg/requestCore/libGin"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

const (
	WebFrameworkKey = libQuery.ContextKey("webFramework")
	Gin             = "gin"
	Fiber           = "fiber"
)

func InitContext(c any) webFramework.WebFramework {
	w := webFramework.WebFramework{}
	switch ctx := c.(type) {
	case *gin.Context:
		w.Ctx = context.WithValue(ctx, WebFrameworkKey, Gin)
		w.Parser = libGin.InitContext(c)
	case *fiber.Ctx:
		w.Ctx = context.WithValue(ctx.Context(), WebFrameworkKey, Fiber)
		w.Parser = libFiber.InitContext(ctx)
	default:
		log.Fatalf("error in InitContext: unknown webFramework %v", ctx)
	}
	w.Ctx = context.WithValue(w.Ctx, libQuery.ContextKey(libQuery.USER), w.Parser.GetHeaderValue("User-Id"))
	return w
}

func InitContextWithHandler(c context.Context, handler response.ResponseHandler) webFramework.WebFramework {
	return InitContext(c)
}
