// Package resources provides v2 resource registration with seven
// standard CRUD operations, inspired by Buffalo's resource pattern.
//
// A Resource defines the seven operations:
//   - List:    GET    /{resource}
//   - Show:    GET    /{resource}/{id}
//   - New:     GET    /{resource}/new
//   - Create:  POST   /{resource}
//   - Edit:    GET    /{resource}/{id}/edit
//   - Update:  PUT    /{resource}/{id}
//   - Destroy: DELETE /{resource}/{id}
//
// PATCH is an optional alias of Update, not a separate seventh operation.
//
// Each operation is independently typed with its own request and response
// types via handlers.Endpoint, providing type safety without requiring a
// single mega-struct. Operations that are not supported return explicit
// 405 responses via ResourceDefaults.
package resources

import (
	"fmt"
	"strconv"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/status"
	"github.com/hmmftg/requestCore/webFramework"

	"github.com/hmmftg/requestCore/v2/handlers"
	v2response "github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/routing"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// Resource defines the seven standard CRUD operations for a resource.
// Each operation returns an Endpoint descriptor that captures its own
// request and response types. Operations returning nil are not supported
// and will be registered with a default 405 handler.
type Resource[ID any] interface {
	// List returns a list of resources.
	List() *handlers.Endpoint
	// Show returns a single resource by ID.
	Show() *handlers.Endpoint
	// New returns the form/data for creating a new resource.
	New() *handlers.Endpoint
	// Create creates a new resource.
	Create() *handlers.Endpoint
	// Edit returns the form/data for editing a resource by ID.
	Edit() *handlers.Endpoint
	// Update replaces a resource by ID.
	Update() *handlers.Endpoint
	// Destroy deletes a resource by ID.
	Destroy() *handlers.Endpoint
}

// ResourceDefaults holds default endpoint handlers for unsupported operations.
// When a Resource returns nil for an operation, the corresponding default
// handler emits a 405 Method Not Allowed response.
type ResourceDefaults struct{}

// CustomOperation defines a non-CRUD action registered alongside a
// resource. Common use cases include Reload, Validate, Approve, or
// other domain-specific actions that don't fit the 7 standard
// operations.
//
// The Path is appended to the resource's base Path. For example, if
// the resource Path is "/parameters" and the CustomOperation Path is
// "/reload", the full route is "POST /parameters/reload".
//
// Custom operations are registered before /{id} routes to ensure
// correct precedence (e.g. "/parameters/reload" won't match "/{id}").
type CustomOperation struct {
	// Method is the HTTP method (GET, POST, PUT, DELETE, PATCH).
	Method string

	// Path is the sub-path appended to the resource base path.
	// Must start with "/". For example: "/reload", "/validate".
	Path string

	// Endpoint is the typed endpoint descriptor for the operation.
	Endpoint *handlers.Endpoint
}

// Config holds the configuration for registering a resource.
type Config[ID any] struct {
	// Path is the base path for the resource (e.g. "/users").
	Path string

	// Resource implements the seven operations.
	Resource Resource[ID]

	// Core is the v1 RequestCoreInterface for infrastructure access.
	// May be nil.
	Core any

	// RespHandler is the v2 response handler.
	RespHandler *v2response.Handler

	// IDParam is the URL parameter name for the resource ID.
	// Default: "id".
	IDParam string

	// IDParser converts a string URL parameter to the ID type.
	// If nil, the string value is used directly for string IDs.
	IDParser func(string) (ID, error)

	// EnablePatchAlias, when true, registers PATCH as an alias for
	// Update on the /{id} path.
	EnablePatchAlias bool

	// Defaults provides 405 handlers for unsupported operations.
	// If nil, unsupported operations are silently skipped.
	Defaults *ResourceDefaults

	// Custom registers non-CRUD operations (e.g. Reload, Validate)
	// alongside the standard 7 operations. Custom operations are
	// registered before /{id} routes for correct path precedence.
	Custom []CustomOperation
}

// Register registers all seven resource operations on the given router.
// Static routes (/new, /{id}/edit) are registered before /{id} routes
// to ensure correct precedence on all adapters.
func Register[ID any](router routing.RouteGroup, config Config[ID]) error {
	idParam := config.IDParam
	if idParam == "" {
		idParam = "id"
	}
	basePath := config.Path
	idPath := basePath + "/{" + idParam + "}"

	// Register static routes first (before /{id}) for correct precedence.
	// New: GET /{resource}/new
	if op := config.Resource.New(); op != nil {
		op = op.WithPath(basePath + "/new")
		if err := handlers.RegisterEndpoint(router, config.Core, config.RespHandler, "GET", basePath+"/new", op); err != nil {
			return err
		}
	} else if config.Defaults != nil {
		if err := registerDefault405(router, config.Defaults, config.RespHandler, "GET", basePath+"/new"); err != nil {
			return err
		}
	}

	// Edit: GET /{resource}/{id}/edit
	if op := config.Resource.Edit(); op != nil {
		op = op.WithPath(basePath + "/{" + idParam + "}/edit")
		op = withIDParser[ID](op, config, idParam)
		if err := handlers.RegisterEndpoint(router, config.Core, config.RespHandler, "GET", basePath+"/{"+idParam+"}/edit", op); err != nil {
			return err
		}
	} else if config.Defaults != nil {
		if err := registerDefault405(router, config.Defaults, config.RespHandler, "GET", basePath+"/{"+idParam+"}/edit"); err != nil {
			return err
		}
	}

	// Custom operations (non-CRUD actions like Reload, Validate).
	// Registered before /{id} routes to ensure correct precedence
	// (e.g. POST /parameters/reload must not match /{id}).
	for _, cop := range config.Custom {
		copPath := basePath + cop.Path
		ep := cop.Endpoint.WithPath(copPath)
		if err := handlers.RegisterEndpoint(router, config.Core, config.RespHandler, cop.Method, copPath, ep); err != nil {
			return fmt.Errorf("resources: custom operation %s %s: %w", cop.Method, copPath, err)
		}
	}

	// List: GET /{resource}
	if op := config.Resource.List(); op != nil {
		op = op.WithPath(basePath)
		if err := handlers.RegisterEndpoint(router, config.Core, config.RespHandler, "GET", basePath, op); err != nil {
			return err
		}
	} else if config.Defaults != nil {
		if err := registerDefault405(router, config.Defaults, config.RespHandler, "GET", basePath); err != nil {
			return err
		}
	}

	// Create: POST /{resource}
	if op := config.Resource.Create(); op != nil {
		op = op.WithPath(basePath)
		if err := handlers.RegisterEndpoint(router, config.Core, config.RespHandler, "POST", basePath, op); err != nil {
			return err
		}
	} else if config.Defaults != nil {
		if err := registerDefault405(router, config.Defaults, config.RespHandler, "POST", basePath); err != nil {
			return err
		}
	}

	// Show: GET /{resource}/{id}
	if op := config.Resource.Show(); op != nil {
		op = op.WithPath(idPath)
		op = withIDParser[ID](op, config, idParam)
		if err := handlers.RegisterEndpoint(router, config.Core, config.RespHandler, "GET", idPath, op); err != nil {
			return err
		}
	} else if config.Defaults != nil {
		if err := registerDefault405(router, config.Defaults, config.RespHandler, "GET", idPath); err != nil {
			return err
		}
	}

	// Update: PUT /{resource}/{id}
	if op := config.Resource.Update(); op != nil {
		op = op.WithPath(idPath)
		op = withIDParser[ID](op, config, idParam)
		if err := handlers.RegisterEndpoint(router, config.Core, config.RespHandler, "PUT", idPath, op); err != nil {
			return err
		}
		// Optional PATCH alias.
		if config.EnablePatchAlias {
			if err := handlers.RegisterEndpoint(router, config.Core, config.RespHandler, "PATCH", idPath, op); err != nil {
				return err
			}
		}
	} else if config.Defaults != nil {
		if err := registerDefault405(router, config.Defaults, config.RespHandler, "PUT", idPath); err != nil {
			return err
		}
	}

	// Destroy: DELETE /{resource}/{id}
	if op := config.Resource.Destroy(); op != nil {
		op = op.WithPath(idPath)
		op = withIDParser[ID](op, config, idParam)
		if err := handlers.RegisterEndpoint(router, config.Core, config.RespHandler, "DELETE", idPath, op); err != nil {
			return err
		}
	} else if config.Defaults != nil {
		if err := registerDefault405(router, config.Defaults, config.RespHandler, "DELETE", idPath); err != nil {
			return err
		}
	}

	return nil
}

// withIDParser wraps an endpoint to parse the ID parameter before the
// handler runs. The parsed ID is stored in the request context's Legacy
// parser locals under the IDParam key.
func withIDParser[ID any](e *handlers.Endpoint, config Config[ID], idParam string) *handlers.Endpoint {
	// We inject ID parsing via WithIDParser, which runs before the
	// initializer and receives the v2 RequestContext directly.
	e.WithIDParser(func(ctx *v2wf.RequestContext) error {
		raw := ctx.Parser.GetURLParam(idParam)
		id, err := parseID[ID](config, raw)
		if err != nil {
			return err
		}
		// Store the parsed ID in locals for the handler to retrieve.
		// Use both ctx.Parser and ctx.Legacy.Parser to ensure the
		// value is accessible regardless of which parser instance
		// the handler reads from.
		ctx.Parser.SetLocal(idParam+"_parsed", id)
		if ctx.Legacy.Parser != nil {
			ctx.Legacy.Parser.SetLocal(idParam+"_parsed", id)
		}
		return nil
	})
	return e
}

// parseID converts a string URL parameter to the ID type.
// If IDParser is configured, it is used. Otherwise, the string is used
// directly for string IDs. For non-string IDs without an IDParser, a
// 400 error is returned.
func parseID[ID any](config Config[ID], raw string) (ID, error) {
	if config.IDParser != nil {
		return config.IDParser(raw)
	}
	var zero ID
	// Try to cast string to ID (works when ID is string).
	if id, ok := any(raw).(ID); ok {
		return id, nil
	}
	// Try int64 conversion.
	if _, ok := any(zero).(int64); ok {
		if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
			if id, ok := any(n).(ID); ok {
				return id, nil
			}
		}
	}
	// Try int conversion.
	if _, ok := any(zero).(int); ok {
		if n, err := strconv.Atoi(raw); err == nil {
			if id, ok := any(n).(ID); ok {
				return id, nil
			}
		}
	}
	return zero, libError.NewWithDescription(
		status.BadRequest,
		"INVALID_ID",
		"invalid id parameter: %s",
		raw,
	)
}

// registerDefault405 registers a default 405 Method Not Allowed handler
// for an unsupported operation.
func registerDefault405(router routing.RouteGroup, defaults *ResourceDefaults, respHandler *v2response.Handler, method, path string) error {
	h := func(ctx *v2wf.RequestContext) error {
		return respHandler.Error(ctx, libError.NewWithDescription(
			status.StatusCode(405),
			"METHOD_NOT_ALLOWED",
			"method %s not allowed on %s",
			method, path,
		))
	}
	return router.Handle(method, path, h)
}

// GetParsedID retrieves a parsed ID from the request context's locals.
// This is used by resource handlers to access the ID parsed by withIDParser.
// The RequestContext is available on the HandlerRequest's V2 field.
func GetParsedID[ID any](ctx *v2wf.RequestContext, idParam string) (ID, error) {
	key := idParam + "_parsed"
	// Check ctx.Parser first (the v2 parser), then ctx.Legacy.Parser.
	var v any
	if ctx.Parser != nil {
		v = ctx.Parser.GetLocal(key)
	}
	if v == nil && ctx.Legacy.Parser != nil {
		v = ctx.Legacy.Parser.GetLocal(key)
	}
	if v == nil {
		var zero ID
		return zero, fmt.Errorf("resources: parsed ID not found for param %q", idParam)
	}
	id, ok := v.(ID)
	if !ok {
		var zero ID
		return zero, fmt.Errorf("resources: parsed ID type mismatch for param %q: got %T", idParam, v)
	}
	return id, nil
}

// Suppress unused import warnings for types used in signatures.
var _ = libRequest.JSON
var _ = webFramework.WebFramework{}
var _ response.ResponseHandler
