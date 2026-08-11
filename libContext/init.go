// Package libContext provides context initialization utilities for requestCore handlers.
package libContext

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/hmmftg/requestCore/libFiber"
	"github.com/hmmftg/requestCore/libGin"
	"github.com/hmmftg/requestCore/libNetHttp"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/webFramework"
)

const (
	// WebFrameworkKey is the context key used to store the web framework type.
	WebFrameworkKey = libQuery.ContextKey("webFramework")
	// Gin identifies the Gin web framework.
	Gin = "gin"
	// Fiber identifies the Fiber web framework.
	Fiber = "fiber"
	// NetHTTP identifies the net/http web framework.
	NetHTTP = "nethttp"
	// Testing identifies the testing framework.
	Testing = "testing"
	// UserIDHeader is the HTTP header name for the user identifier.
	UserIDHeader = "User-Id"
	// UserIDLocal is the local storage key for the user identifier.
	UserIDLocal = "userId"
	// UnknownUser is the default user identifier when none is provided.
	UnknownUser = "unknown"
)

// InitContext initializes a webFramework.WebFramework from the given framework-specific context.
func InitContext(c any) webFramework.WebFramework {
	return initContext(c, false)
}

// InitContextNoAuditTrail initializes a webFramework.WebFramework without audit trail.
// It is useful in Get handlers which mostly don't have audit trail.
func InitContextNoAuditTrail(c any) webFramework.WebFramework {
	return initContext(c, true)
}
func initContext(c any, unknownUser bool) webFramework.WebFramework {
	w := webFramework.WebFramework{}
	var span trace.Span

	switch ctx := c.(type) {
	case *gin.Context:
		if unknownUser {
			ctx.Set(UserIDLocal, UnknownUser)
		}
		w.Ctx = context.WithValue(ctx, WebFrameworkKey, Gin)
		w.Parser = libGin.InitContext(c)
		// Extract trace context from Gin context
		span = trace.SpanFromContext(ctx)
	case *fiber.Ctx:
		if unknownUser {
			ctx.Locals(UserIDLocal, UnknownUser)
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
			fiberCtx.Locals(UserIDLocal, UnknownUser)
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
	case context.Context:
		req, okReq := libNetHttp.RequestFromContext(ctx)
		writer, okWriter := libNetHttp.ResponseWriterFromContext(ctx)
		if okReq && okWriter {
			return InitNetHTTPContext(req, writer, unknownUser)
		}
		stack := response.GetStack(1, "libContext/init.go")
		log.Fatalf("error in InitContext: context missing net/http request/response data %T, Stack: %s", ctx, stack)
	default:
		stack := response.GetStack(1, "libContext/init.go")
		log.Fatalf("error in InitContext: unknown webFramework %T, Stack: %s", ctx, stack)
	}

	// Set span in WebFramework
	w.Span = span

	userID := w.Parser.GetHeaderValue(UserIDHeader)
	if len(userID) == 0 {
		userID = w.Parser.GetLocalString(UserIDLocal)
	}
	if len(userID) == 0 {
		stack := response.GetStack(1, "libContext/init.go")
		webFramework.AddLog(w, webFramework.HandlerLogTag,
			slog.Group("unable to find userId in header and locals => audit trail will fail", slog.String("title", stack)))
	}
	w.Ctx = context.WithValue(w.Ctx, libQuery.ContextKey(libQuery.User), userID)

	// Add tracing attributes if span is available
	if span != nil && span.IsRecording() {
		span.SetAttributes(
			attribute.String("user.id", userID),
			attribute.String("framework", getFrameworkName(c)),
		)
	}

	return w
}

// InitContextWithHandler initializes a webFramework.WebFramework from the given context and response handler.
func InitContextWithHandler(c context.Context, _ response.ResponseHandler) webFramework.WebFramework {
	return InitContext(c)
}

// InitNetHTTPContext initializes a webFramework.WebFramework for the net/http framework.
func InitNetHTTPContext(r *http.Request, w http.ResponseWriter, unknownUser bool) webFramework.WebFramework {
	wf := webFramework.WebFramework{}

	// Create net/http parser
	netHTTPCtx := libNetHttp.InitContext(r, w)

	// Set unknown user if needed
	if unknownUser {
		netHTTPCtx.SetLocal(UserIDLocal, UnknownUser)
	}

	// Set framework context
	wf.Ctx = context.WithValue(r.Context(), WebFrameworkKey, NetHTTP)
	wf.Parser = netHTTPCtx

	// Extract trace context from request
	span := trace.SpanFromContext(r.Context())
	wf.Span = span

	// Extract user ID
	userID := wf.Parser.GetHeaderValue(UserIDHeader)
	if len(userID) == 0 {
		userID = wf.Parser.GetLocalString(UserIDLocal)
	}
	if len(userID) == 0 {
		stack := response.GetStack(1, "libContext/init.go")
		webFramework.AddLog(wf, webFramework.HandlerLogTag,
			slog.Group("unable to find userId in header and locals => audit trail will fail", slog.String("title", stack)))
	}
	wf.Ctx = context.WithValue(wf.Ctx, libQuery.ContextKey(libQuery.User), userID)

	// Add tracing attributes if span is available
	if span != nil && span.IsRecording() {
		span.SetAttributes(
			attribute.String("user.id", userID),
			attribute.String("framework", NetHTTP),
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
