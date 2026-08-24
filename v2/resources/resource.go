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
// types via handlers.Endpoint[Req, Resp], providing compile-time type
// safety. Operations that are not supported return nil and are registered
// with a default 405 response via ResourceDefaults.
//
// Two resource interfaces are provided:
//   - Resource[ID]: the recommended interface for v2 migration. Each
//     operation returns handlers.EndpointRuntime. Implement this for
//     simple CRUD + custom (non-CRUD) resources. Pair with
//     ResourceBuilder for fluent registration.
//   - TypedResource[ID, ...]: advanced interface with per-operation type
//     parameters (14 type params). Each operation returns a fully typed
//     *handlers.Endpoint[Req, Resp]. This is overkill for most resources
//     — use it only when you need the strictest compile-time guarantees
//     on every operation's request/response types. Any TypedResource
//     automatically satisfies Resource[ID].
//
// For v2 migration, the recommended path is:
//
//	resources.NewResource[string]("/users").            // ResourceBuilder
//	    EnablePatch().
//	    WithCustom(reloadOp).
//	    Register(router, core, respHandler, &UserResource{}) // implements Resource[ID]
//
// Avoid TypedResource unless you have a specific need for 14-type-parameter
// strictness — the verbosity outweighs the benefit for simple CRUD resources.
package resources

import (
	"cmp"
	"fmt"
	"strconv"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/status"

	"github.com/hmmftg/requestCore/v2/handlers"
	v2response "github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/routing"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// Resource defines the seven standard CRUD operations for a resource.
// Each operation returns an EndpointRuntime — a type-erased interface
// that can be registered on a router. Operations returning nil are not
// supported and will be registered with a default 405 handler.
//
// ID must be cmp.Ordered (string, int, int64, float64, etc.) to ensure
// IDs can be compared and sorted at compile time.
type Resource[ID cmp.Ordered] interface {
	// List returns a list of resources.
	List() handlers.EndpointRuntime
	// Show returns a single resource by ID.
	Show() handlers.EndpointRuntime
	// New returns the form/data for creating a new resource.
	New() handlers.EndpointRuntime
	// Create creates a new resource.
	Create() handlers.EndpointRuntime
	// Edit returns the form/data for editing a resource by ID.
	Edit() handlers.EndpointRuntime
	// Update replaces a resource by ID.
	Update() handlers.EndpointRuntime
	// Destroy deletes a resource by ID.
	Destroy() handlers.EndpointRuntime
}

// TypedResource is an advanced resource interface with per-operation type
// parameters. Implementing this interface gives compile-time type safety
// for all 7 operations — each operation returns a fully typed
// *handlers.Endpoint[Req, Resp].
//
// Any TypedResource automatically satisfies Resource[ID] because
// *handlers.Endpoint[Req, Resp] implements handlers.EndpointRuntime.
//
// The 14 type parameters (7 request + 7 response types) make this
// verbose to spell out. For most v2 migration use cases (simple CRUD +
// custom operations like Reload), prefer Resource[ID] with
// ResourceBuilder — the 14 type parameters are overkill. Use
// TypedResource only when you need the strictest compile-time guarantees
// on every operation's request/response types simultaneously.
type TypedResource[
	ID cmp.Ordered,
	ListReq, ListResp any,
	ShowReq, ShowResp any,
	NewReq, NewResp any,
	CreateReq, CreateResp any,
	EditReq, EditResp any,
	UpdateReq, UpdateResp any,
	DestroyReq, DestroyResp any,
] interface {
	List() *handlers.Endpoint[ListReq, ListResp]
	Show() *handlers.Endpoint[ShowReq, ShowResp]
	New() *handlers.Endpoint[NewReq, NewResp]
	Create() *handlers.Endpoint[CreateReq, CreateResp]
	Edit() *handlers.Endpoint[EditReq, EditResp]
	Update() *handlers.Endpoint[UpdateReq, UpdateResp]
	Destroy() *handlers.Endpoint[DestroyReq, DestroyResp]
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
	Endpoint handlers.EndpointRuntime
}

// Config holds the configuration for registering a resource.
type Config[ID cmp.Ordered] struct {
	// Path is the base path for the resource (e.g. "/users").
	Path string

	// Resource implements the seven operations.
	Resource Resource[ID]

	// Core is the v1 RequestCoreInterface for infrastructure access.
	// May be nil for pure v2 applications.
	Core requestCore.RequestCoreInterface

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
func Register[ID cmp.Ordered](router routing.RouteGroup, config Config[ID]) error {
	idParam := config.IDParam
	if idParam == "" {
		idParam = "id"
	}
	basePath := config.Path
	idPath := basePath + "/{" + idParam + "}"

	// Register static routes first (before /{id}) for correct precedence.
	// New: GET /{resource}/new
	if op := config.Resource.New(); op != nil {
		setEndpointPath(op, basePath+"/new")
		if err := handlers.RegisterRuntime(router, config.Core, config.RespHandler, "GET", basePath+"/new", op); err != nil {
			return err
		}
	} else if config.Defaults != nil {
		if err := registerDefault405(router, config.Defaults, config.RespHandler, "GET", basePath+"/new"); err != nil {
			return err
		}
	}

	// Edit: GET /{resource}/{id}/edit
	if op := config.Resource.Edit(); op != nil {
		setEndpointPath(op, basePath+"/{"+idParam+"}/edit")
		withIDParser[ID](op, config, idParam)
		if err := handlers.RegisterRuntime(router, config.Core, config.RespHandler, "GET", basePath+"/{"+idParam+"}/edit", op); err != nil {
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
		setEndpointPath(cop.Endpoint, copPath)
		if err := handlers.RegisterRuntime(router, config.Core, config.RespHandler, cop.Method, copPath, cop.Endpoint); err != nil {
			return fmt.Errorf("resources: custom operation %s %s: %w", cop.Method, copPath, err)
		}
	}

	// List: GET /{resource}
	if op := config.Resource.List(); op != nil {
		setEndpointPath(op, basePath)
		if err := handlers.RegisterRuntime(router, config.Core, config.RespHandler, "GET", basePath, op); err != nil {
			return err
		}
	} else if config.Defaults != nil {
		if err := registerDefault405(router, config.Defaults, config.RespHandler, "GET", basePath); err != nil {
			return err
		}
	}

	// Create: POST /{resource}
	if op := config.Resource.Create(); op != nil {
		setEndpointPath(op, basePath)
		if err := handlers.RegisterRuntime(router, config.Core, config.RespHandler, "POST", basePath, op); err != nil {
			return err
		}
	} else if config.Defaults != nil {
		if err := registerDefault405(router, config.Defaults, config.RespHandler, "POST", basePath); err != nil {
			return err
		}
	}

	// Show: GET /{resource}/{id}
	if op := config.Resource.Show(); op != nil {
		setEndpointPath(op, idPath)
		withIDParser[ID](op, config, idParam)
		if err := handlers.RegisterRuntime(router, config.Core, config.RespHandler, "GET", idPath, op); err != nil {
			return err
		}
	} else if config.Defaults != nil {
		if err := registerDefault405(router, config.Defaults, config.RespHandler, "GET", idPath); err != nil {
			return err
		}
	}

	// Update: PUT /{resource}/{id}
	if op := config.Resource.Update(); op != nil {
		setEndpointPath(op, idPath)
		withIDParser[ID](op, config, idParam)
		if err := handlers.RegisterRuntime(router, config.Core, config.RespHandler, "PUT", idPath, op); err != nil {
			return err
		}
		// Optional PATCH alias.
		if config.EnablePatchAlias {
			if err := handlers.RegisterRuntime(router, config.Core, config.RespHandler, "PATCH", idPath, op); err != nil {
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
		setEndpointPath(op, idPath)
		withIDParser[ID](op, config, idParam)
		if err := handlers.RegisterRuntime(router, config.Core, config.RespHandler, "DELETE", idPath, op); err != nil {
			return err
		}
	} else if config.Defaults != nil {
		if err := registerDefault405(router, config.Defaults, config.RespHandler, "DELETE", idPath); err != nil {
			return err
		}
	}

	return nil
}

// setEndpointPath sets the path on an EndpointRuntime if it also
// implements ConfigurableEndpoint. Endpoints that don't support
// configuration are left unchanged.
func setEndpointPath(ep handlers.EndpointRuntime, path string) {
	if setter, ok := ep.(handlers.ConfigurableEndpoint); ok {
		setter.SetPath(path)
	}
}

// withIDParser injects an ID parser into an EndpointRuntime if it
// also implements ConfigurableEndpoint. The parsed ID is stored in
// the request context's parser locals under the IDParam key.
func withIDParser[ID cmp.Ordered](ep handlers.EndpointRuntime, config Config[ID], idParam string) {
	setter, ok := ep.(handlers.ConfigurableEndpoint)
	if !ok {
		return
	}
	setter.SetIDParser(func(ctx *v2wf.RequestContext) error {
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
}

// parseID converts a string URL parameter to the ID type.
// If IDParser is configured, it is used. Otherwise, the string is used
// directly for string IDs. For non-string IDs without an IDParser, a
// 400 error is returned.
func parseID[ID cmp.Ordered](config Config[ID], raw string) (ID, error) {
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
func GetParsedID[ID cmp.Ordered](ctx *v2wf.RequestContext, idParam string) (ID, error) {
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

// ResourceBuilder provides a fluent API for constructing and registering
// resources. It is the recommended way to register resources in v2
// application code — prefer it over passing raw Config[ID] to Register.
//
// The typical v2 migration path is:
//
//	resources.NewResource[string]("/users").
//	    EnablePatch().
//	    WithCustom(reloadOp).
//	    Register(router, core, respHandler, &UserResource{})
//
// where UserResource implements Resource[string] (not TypedResource).
type ResourceBuilder[ID cmp.Ordered] struct {
	path       string
	idParam    string
	idParser   func(string) (ID, error)
	patchAlias bool
	defaults   *ResourceDefaults
	customs    []CustomOperation
}

// NewResource creates a ResourceBuilder for the given base path.
// The ID type parameter must be cmp.Ordered (string, int, int64, etc.).
func NewResource[ID cmp.Ordered](path string) *ResourceBuilder[ID] {
	return &ResourceBuilder[ID]{path: path, idParam: "id"}
}

// WithIDParam sets the URL parameter name for the resource ID.
// Default: "id".
func (b *ResourceBuilder[ID]) WithIDParam(name string) *ResourceBuilder[ID] {
	b.idParam = name
	return b
}

// WithIDParser sets a custom ID parser function.
func (b *ResourceBuilder[ID]) WithIDParser(fn func(string) (ID, error)) *ResourceBuilder[ID] {
	b.idParser = fn
	return b
}

// EnablePatch enables PATCH as an alias for Update on the /{id} path.
func (b *ResourceBuilder[ID]) EnablePatch() *ResourceBuilder[ID] {
	b.patchAlias = true
	return b
}

// WithDefaults sets the ResourceDefaults for 405 handlers on
// unsupported operations.
func (b *ResourceBuilder[ID]) WithDefaults(d *ResourceDefaults) *ResourceBuilder[ID] {
	b.defaults = d
	return b
}

// WithCustom adds custom (non-CRUD) operations to the resource.
func (b *ResourceBuilder[ID]) WithCustom(ops ...CustomOperation) *ResourceBuilder[ID] {
	b.customs = append(b.customs, ops...)
	return b
}

// Register registers all resource operations on the given router using
// the builder's configuration.
func (b *ResourceBuilder[ID]) Register(
	router routing.RouteGroup,
	core requestCore.RequestCoreInterface,
	respHandler *v2response.Handler,
	resource Resource[ID],
) error {
	return Register(router, Config[ID]{
		Path:             b.path,
		Resource:         resource,
		Core:             core,
		RespHandler:      respHandler,
		IDParam:          b.idParam,
		IDParser:         b.idParser,
		EnablePatchAlias: b.patchAlias,
		Defaults:         b.defaults,
		Custom:           b.customs,
	})
}

// Suppress unused import warnings for types used in signatures.
var _ = libRequest.JSON
