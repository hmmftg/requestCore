package routing

import (
	"github.com/hmmftg/requestCore/v2/webFramework"
)

// AdaptLegacy wraps a v1 handler function (func(context.Context)) into a
// v2 Handler. The v1 handler is invoked with the RequestContext's
// LegacyContext, which is the framework-native context expected by
// libContext.InitContext.
//
// This adapter does NOT expose v2 session, worker, or renderer features
// to the legacy handler. The legacy handler runs on the v1 lifecycle.
// Errors from the legacy handler are not captured (v1 handlers typically
// write responses directly and do not return errors).
func AdaptLegacy(legacyHandler func(any)) Handler {
	return func(ctx *webFramework.RequestContext) error {
		legacyHandler(ctx.LegacyContext)
		return nil
	}
}

// AdaptLegacyWithError wraps a v1 handler function that returns an error.
// If the legacy handler returns a non-nil error, it is propagated to the
// v2 error handler registry.
func AdaptLegacyWithError(legacyHandler func(any) error) Handler {
	return func(ctx *webFramework.RequestContext) error {
		return legacyHandler(ctx.LegacyContext)
	}
}
