package libContext

import (
	"context"
	"log"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
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

func InitContext(c any) webFramework.WebFramework {
	w := webFramework.WebFramework{}
	switch ctx := c.(type) {
	case *gin.Context:
		w.Ctx = context.WithValue(ctx, WebFrameworkKey, Gin)
		w.Parser = libGin.InitContext(c)
	case *fiber.Ctx:
		w.Ctx = context.WithValue(ctx.Context(), WebFrameworkKey, Fiber)
		w.Parser = libFiber.InitContext(ctx)
	case *fasthttp.RequestCtx:
		fiberCtx, ok := ctx.UserValue(libFiber.FiberCtxKey).(*fiber.Ctx)
		if !ok {
			stack := debug.Stack()
			log.Fatalf("error in InitContext: unable to parse fiber ctx %T, Stack: %s", ctx.UserValue(libFiber.FiberCtxKey), string(stack))
		}
		w.Ctx = context.WithValue(ctx, WebFrameworkKey, Fiber)
		w.Parser = libFiber.InitContext(fiberCtx)
	default:
		stack := debug.Stack()
		log.Fatalf("error in InitContext: unknown webFramework %T, Stack: %s", ctx, string(stack))
	}
	userId := w.Parser.GetHeaderValue("User-Id")
	if len(userId) == 0 {
		userId = w.Parser.GetLocalString("user")
	}
	if len(userId) == 0 {
		log.Println("unable to find userId in header and locals => audit trail will fail")
	}
	w.Ctx = context.WithValue(w.Ctx, libQuery.ContextKey(libQuery.USER), userId)
	return w
}

func InitContextWithHandler(c context.Context, handler response.ResponseHandler) webFramework.WebFramework {
	return InitContext(c)
}
