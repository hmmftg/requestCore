package routing

// legacy.go previously contained AdaptLegacy and AdaptLegacyWithError,
// which wrapped v1 handler functions into v2 Handlers via
// webFramework.RequestContext. These adapters have been removed in
// Tranche 4 Phase 2 because the routing contract now uses
// (*request.Context, Transport) instead of *webFramework.RequestContext.
// Callers should migrate to the canonical handler signature:
//
//	func(*request.Context, Req) (Resp, error)
