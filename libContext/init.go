package libContext

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
	"github.com/hmmftg/requestCore/libFiber"
	"github.com/hmmftg/requestCore/libGin"
	"github.com/hmmftg/requestCore/libNetHttp"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
	"github.com/valyala/fasthttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	WebFrameworkKey = libQuery.ContextKey("webFramework")
	Gin             = "gin"
	Fiber           = "fiber"
	NetHttp         = "nethttp"
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
	var span trace.Span

	switch ctx := c.(type) {
	case *gin.Context:
		if unknownUser {
			ctx.Set(UserIdLocal, UnknownUser)
		}
		w.Ctx = context.WithValue(ctx, WebFrameworkKey, Gin)
		w.Parser = libGin.InitContext(c)
		// Extract trace context from Gin context
		span = trace.SpanFromContext(ctx)
	case *fiber.Ctx:
		if unknownUser {
			ctx.Locals(UserIdLocal, UnknownUser)
		}
		w.Ctx = context.WithValue(ctx.Context(), WebFrameworkKey, Fiber)
		w.Parser = libFiber.InitContext(ctx)
		// Extract trace context from Fiber context
		span = trace.SpanFromContext(ctx.Context())
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
		// Extract trace context from Fiber context
		span = trace.SpanFromContext(fiberCtx.Context())
	case *testing.T:
		w.Ctx = context.WithValue(context.Background(), WebFrameworkKey, Testing)
		w.Parser = initTestContext(ctx)
		// No tracing in test context
		span = nil
	default:
		stack := response.GetStack(1, "libContext/init.go")
		log.Fatalf("error in InitContext: unknown webFramework %T, Stack: %s", ctx, stack)
	}

	// Set span in WebFramework
	w.Span = span

	userId := w.Parser.GetHeaderValue(UserIdHeader)
	if len(userId) == 0 {
		userId = w.Parser.GetLocalString(UserIdLocal)
	}
	if len(userId) == 0 {
		stack := response.GetStack(1, "libContext/init.go")
		webFramework.AddLog(w, webFramework.HandlerLogTag,
			slog.Group("unable to find userId in header and locals => audit trail will fail", slog.String("title", stack)))
	}
	w.Ctx = context.WithValue(w.Ctx, libQuery.ContextKey(libQuery.USER), userId)

	// Add tracing attributes if span is available
	if span != nil && span.IsRecording() {
		span.SetAttributes(
			attribute.String("user.id", userId),
			attribute.String("framework", getFrameworkName(c)),
		)
	}

	return w
}

func InitContextWithHandler(c context.Context, handler response.ResponseHandler) webFramework.WebFramework {
	return InitContext(c)
}

// InitNetHttpContext initializes context for net/http framework
func InitNetHttpContext(r *http.Request, w http.ResponseWriter, unknownUser bool) webFramework.WebFramework {
	wf := webFramework.WebFramework{}

	// Create net/http parser
	netHttpCtx := libNetHttp.InitContext(r, w)

	// Set unknown user if needed
	if unknownUser {
		netHttpCtx.SetLocal(UserIdLocal, UnknownUser)
	}

	// Set framework context
	wf.Ctx = context.WithValue(r.Context(), WebFrameworkKey, NetHttp)
	wf.Parser = netHttpCtx

	// Extract trace context from request
	span := trace.SpanFromContext(r.Context())
	wf.Span = span

	// Extract user ID
	userId := wf.Parser.GetHeaderValue(UserIdHeader)
	if len(userId) == 0 {
		userId = wf.Parser.GetLocalString(UserIdLocal)
	}
	if len(userId) == 0 {
		stack := response.GetStack(1, "libContext/init.go")
		webFramework.AddLog(wf, webFramework.HandlerLogTag,
			slog.Group("unable to find userId in header and locals => audit trail will fail", slog.String("title", stack)))
	}
	wf.Ctx = context.WithValue(wf.Ctx, libQuery.ContextKey(libQuery.USER), userId)

	// Add tracing attributes if span is available
	if span != nil && span.IsRecording() {
		span.SetAttributes(
			attribute.String("user.id", userId),
			attribute.String("framework", NetHttp),
		)
	}

	return wf
}

// getFrameworkName returns the framework name for tracing
func getFrameworkName(c any) string {
	switch c.(type) {
	case *gin.Context:
		return Gin
	case *fiber.Ctx:
		return Fiber
	case *fasthttp.RequestCtx:
		return Fiber
	case *testing.T:
		return Testing
	default:
		return "unknown"
	}
}
