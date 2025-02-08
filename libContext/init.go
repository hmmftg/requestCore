package libContext

import (
	"context"
	"log"
	"log/slog"
	"testing"

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
	Testing         = "testing"
	UserIdHeader    = "User-Id"
	UserIdLocal     = "userId"
	UnknownUser     = "unknown"
)

func InitContext(c any) webFramework.WebFramework {
	return initContext(c, false)
}

// useful in Get handlers which mostly don't have audit trail
func InitContextNoAuditTrail(c any) webFramework.WebFramework {
	return initContext(c, true)
}
func initContext(c any, unknownUser bool) webFramework.WebFramework {
	w := webFramework.WebFramework{}
	switch ctx := c.(type) {
	case *gin.Context:
		if unknownUser {
			ctx.Set(UserIdLocal, UnknownUser)
		}
		w.Ctx = context.WithValue(ctx, WebFrameworkKey, Gin)
		w.Parser = libGin.InitContext(c)
	case *fiber.Ctx:
		if unknownUser {
			ctx.Locals(UserIdLocal, UnknownUser)
		}
		w.Ctx = context.WithValue(ctx.Context(), WebFrameworkKey, Fiber)
		w.Parser = libFiber.InitContext(ctx)
	case *fasthttp.RequestCtx:
		fiberCtx, ok := ctx.UserValue(libFiber.FiberCtxKey).(*fiber.Ctx)
		if !ok {
			stack := response.GetStack(1, "libContext/init.go")
			log.Fatalf("error in InitContext: unable to parse fiber ctx %T, Stack: %s", ctx.UserValue(libFiber.FiberCtxKey), stack)
		}
		if unknownUser {
			fiberCtx.Locals(UserIdLocal, UnknownUser)
		}
		w.Ctx = context.WithValue(ctx, WebFrameworkKey, Fiber)
		w.Parser = libFiber.InitContext(fiberCtx)
	case *testing.T:
		w.Ctx = context.WithValue(context.Background(), WebFrameworkKey, Testing)
		w.Parser = initTestContext(ctx)
	default:
		stack := response.GetStack(1, "libContext/init.go")
		log.Fatalf("error in InitContext: unknown webFramework %T, Stack: %s", ctx, stack)
	}
	userId := w.Parser.GetHeaderValue(UserIdHeader)
	if len(userId) == 0 {
		userId = w.Parser.GetLocalString(UserIdLocal)
	}
	if len(userId) == 0 {
		stack := response.GetStack(1, "libContext/init.go")
		slog.Error("unable to find userId in header and locals => audit trail will fail", slog.String("title", stack))
	}
	w.Ctx = context.WithValue(w.Ctx, libQuery.ContextKey(libQuery.USER), userId)
	return w
}

func InitContextWithHandler(c context.Context, handler response.ResponseHandler) webFramework.WebFramework {
	return InitContext(c)
}
