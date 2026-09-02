// Package nextadapter bridges the new internal endpoint engine into the
// v2 routing layer. It is fully internal.
//
// The bridge converts a routing.Transport into an endpoint.Transport,
// runs the internal endpoint executor, and returns any error to the
// router's error handler.
package nextadapter

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/hmmftg/requestCore/v2/internal/endpoint"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/routing"
)

// transportAdapter wraps a routing.Transport to implement
// endpoint.Transport. Since routing.Transport is structurally
// compatible with endpoint.Transport, this is a thin pass-through.
type transportAdapter struct {
	transport routing.Transport
}

func (t *transportAdapter) WriteResponse(status int, contentType string, headers http.Header, body []byte) error {
	return t.transport.WriteResponse(status, contentType, headers, body)
}

func (t *transportAdapter) Committed() bool {
	return t.transport.Committed()
}

// Wrap converts a new-kernel internal endpoint into a routing.Handler.
// The returned handler:
//
//  1. Receives *request.Context and routing.Transport from the router.
//  2. Wraps the transport as endpoint.Transport.
//  3. Calls endpoint.Execute.
//  4. Returns the error (if any) for the router's error handler.
func Wrap[Req, Resp any](ep *endpoint.Endpoint[Req, Resp], exec *endpoint.Executor) routing.Handler {
	return func(ctx *request.Context, transport routing.Transport) error {
		if ep == nil {
			return errors.New("nextadapter: nil endpoint")
		}
		if exec == nil {
			return errors.New("nextadapter: nil executor")
		}
		if ctx == nil {
			return errors.New("nextadapter: nil context")
		}
		if transport == nil {
			return errors.New("nextadapter: nil transport")
		}

		epTransport := &transportAdapter{transport: transport}
		_, err := endpoint.Execute(exec, ctx, ep, epTransport)
		return err
	}
}

// Register registers a new-kernel endpoint's operation metadata with
// the executor's registry and registers the wrapped handler on the
// route group.
//
// Pre-validation removes expected failures before mutation:
//   - non-nil dependencies;
//   - canonical route pattern;
//   - operation ID, method, and pattern explicitly present;
//   - operation method/pattern match the supplied registration
//     method/path (immutable metadata is validated, not rewritten).
//
// If router registration fails after metadata registration, the
// operation is unregistered (rolled back) to maintain consistency.
func Register[Req, Resp any](
	router routing.RouteGroup,
	exec *endpoint.Executor,
	method, path string,
	ep *endpoint.Endpoint[Req, Resp],
) error {
	if router == nil {
		return errors.New("nextadapter: nil router")
	}
	if exec == nil {
		return errors.New("nextadapter: nil executor")
	}
	if ep == nil {
		return errors.New("nextadapter: nil endpoint")
	}
	if err := routing.ValidatePattern(path); err != nil {
		return fmt.Errorf("nextadapter: invalid pattern: %w", err)
	}
	op := ep.Config.Operation
	if op.ID == "" {
		return errors.New("nextadapter: operation ID is required")
	}
	if op.Method == "" {
		return errors.New("nextadapter: operation method is required")
	}
	if op.Pattern == "" {
		return errors.New("nextadapter: operation pattern is required")
	}
	if op.Method != method {
		return fmt.Errorf("nextadapter: operation method %q does not match registration method %q", op.Method, method)
	}
	if op.Pattern != path {
		return fmt.Errorf("nextadapter: operation pattern %q does not match registration path %q", op.Pattern, path)
	}

	// Register operation metadata first.
	if err := endpoint.Register(exec, ep); err != nil {
		return fmt.Errorf("nextadapter: register operation: %w", err)
	}

	// Register the wrapped handler on the route group.
	if err := router.Handle(method, path, Wrap[Req, Resp](ep, exec)); err != nil {
		// Rollback: unregister the operation metadata if router
		// registration fails.
		_ = exec.Registry.Unregister(op.ID)
		return fmt.Errorf("nextadapter: register route: %w", err)
	}
	return nil
}
