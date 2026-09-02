// Package nextadapter: context_bridge.go previously contained
// buildContext, which converted v2-alpha RequestContext to
// *request.Context. This is no longer needed because routing now
// provides *request.Context directly.
package nextadapter
