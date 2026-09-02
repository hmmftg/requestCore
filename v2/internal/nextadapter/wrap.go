// Package nextadapter is the internal alias for the public adapter
// package. It re-exports Wrap and Register for backward compatibility
// during the Tranche 4 migration. Callers should switch to
// github.com/hmmftg/requestCore/v2/adapter directly.
package nextadapter

import (
	"github.com/hmmftg/requestCore/v2/adapter"
	"github.com/hmmftg/requestCore/v2/endpoint"
	"github.com/hmmftg/requestCore/v2/routing"
)

// Wrap re-exports adapter.Wrap.
func Wrap[Req, Resp any](ep *endpoint.Endpoint[Req, Resp], exec *endpoint.Executor) routing.Handler {
	return adapter.Wrap[Req, Resp](ep, exec)
}

// Register re-exports adapter.Register.
func Register[Req, Resp any](
	router routing.RouteGroup,
	exec *endpoint.Executor,
	method, path string,
	ep *endpoint.Endpoint[Req, Resp],
) error {
	return adapter.Register[Req, Resp](router, exec, method, path, ep)
}
