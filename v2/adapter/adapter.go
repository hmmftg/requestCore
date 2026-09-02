// Package adapter adapts typed endpoints and mapped errors to routing
// handlers. It is the canonical public bridge between the endpoint
// executor and the routing layer.
//
// Wrap accepts a routing.Transport directly and delegates to
// endpoint.Execute. Register performs validated metadata registration
// with rollback on router registration failure. MappedErrorHandler
// provides a routing.ErrorHandler that converts errors to RFC 9457
// Problems via a response.MapperRegistry.
package adapter

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/hmmftg/requestCore/v2/endpoint"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/response"
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

// Wrap converts a typed endpoint into a routing.Handler. The returned
// handler receives *request.Context and routing.Transport from the
// router, wraps the transport as endpoint.Transport, calls
// endpoint.Execute, and returns any error for the router's error
// handler.
func Wrap[Req, Resp any](ep *endpoint.Endpoint[Req, Resp], exec *endpoint.Executor) routing.Handler {
	return func(ctx *request.Context, transport routing.Transport) error {
		if ep == nil {
			return errors.New("adapter: nil endpoint")
		}
		if exec == nil {
			return errors.New("adapter: nil executor")
		}
		if ctx == nil {
			return errors.New("adapter: nil context")
		}
		if transport == nil {
			return errors.New("adapter: nil transport")
		}

		epTransport := &transportAdapter{transport: transport}
		_, err := endpoint.Execute(exec, ctx, ep, epTransport)
		return err
	}
}

// Register registers a typed endpoint's operation metadata with the
// executor's registry and registers the wrapped handler on the route
// group.
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
		return errors.New("adapter: nil router")
	}
	if exec == nil {
		return errors.New("adapter: nil executor")
	}
	if ep == nil {
		return errors.New("adapter: nil endpoint")
	}
	if err := routing.ValidatePattern(path); err != nil {
		return fmt.Errorf("adapter: invalid pattern: %w", err)
	}
	op := ep.Config.Operation
	if op.ID == "" {
		return errors.New("adapter: operation ID is required")
	}
	if op.Method == "" {
		return errors.New("adapter: operation method is required")
	}
	if op.Pattern == "" {
		return errors.New("adapter: operation pattern is required")
	}
	if op.Method != method {
		return fmt.Errorf("adapter: operation method %q does not match registration method %q", op.Method, method)
	}
	if op.Pattern != path {
		return fmt.Errorf("adapter: operation pattern %q does not match registration path %q", op.Pattern, path)
	}

	// Register operation metadata first.
	if err := endpoint.Register(exec, ep); err != nil {
		return fmt.Errorf("adapter: register operation: %w", err)
	}

	// Register the wrapped handler on the route group.
	if err := router.Handle(method, path, Wrap[Req, Resp](ep, exec)); err != nil {
		// Rollback: unregister the operation metadata if router
		// registration fails.
		_ = exec.Registry.Unregister(op.ID)
		return fmt.Errorf("adapter: register route: %w", err)
	}
	return nil
}

// MappedErrorHandler creates a routing.ErrorHandler that converts
// errors to RFC 9457 Problems via the given MapperRegistry and writes
// them through the transport as application/problem+json.
func MappedErrorHandler(mapper *response.MapperRegistry) routing.ErrorHandler {
	return func(ctx *request.Context, transport routing.Transport, err error) {
		if mapper == nil || err == nil {
			return
		}
		problem := mapper.Map(err)
		if problem == nil {
			return
		}
		body, encErr := problem.MarshalJSON()
		if encErr != nil {
			return
		}
		headers := http.Header{}
		headers.Set("Content-Type", "application/problem+json")
		_ = transport.WriteResponse(problem.Status, "application/problem+json", headers, body)
	}
}
