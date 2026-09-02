package handlers

import (
	"github.com/hmmftg/requestCore/v2/request"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// bridgeRequestContext constructs a v2wf.RequestContext from a new-kernel
// *request.Context. This is a temporary bridge that will be removed in
// Phase 6 when handlers are rewritten to use *request.Context directly.
//
// The native context (gin.Context, fiber.Ctx, *http.Request) is stored
// as LegacyContext so libContext.InitContext can initialize the v1
// parser/tracing pipeline.
func bridgeRequestContext(ctx *request.Context) *v2wf.RequestContext {
	rc := &v2wf.RequestContext{
		Context:       ctx.Context(),
		LegacyContext: ctx.Native(),
	}
	return rc
}
