// Package resources provides v2 resource registration with seven
// standard CRUD operations, inspired by Buffalo's resource pattern.
//
// A Resource defines the seven operations:
//   - Index:    GET    /{resource}
//   - Show:     GET    /{resource}/{id}
//   - Create:   POST   /{resource}
//   - Update:   PUT    /{resource}/{id}
//   - Patch:    PATCH  /{resource}/{id}
//   - Destroy:  DELETE /{resource}/{id}
//   - New:      GET    /{resource}/new
//
// Each operation is independently typed with its own request and response
// types, providing type safety without requiring a single mega-struct.
package resources

import (
	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libRequest"

	v2response "github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/routing"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// Resource defines the seven standard CRUD operations for a resource.
// Each operation has its own request and response types.
type Resource[ID any] interface {
	// Index returns a list of resources.
	Index() IndexOperation
	// Show returns a single resource by ID.
	Show() ShowOperation[ID]
	// Create creates a new resource.
	Create() CreateOperation
	// Update replaces a resource by ID.
	Update() UpdateOperation[ID]
	// Patch partially updates a resource by ID.
	Patch() PatchOperation[ID]
	// Destroy deletes a resource by ID.
	Destroy() DestroyOperation[ID]
	// New returns the form/data for creating a new resource.
	New() NewOperation
}

// IndexOperation lists resources.
type IndexOperation struct {
	Title   string
	Handler func(trx *ResourceContext) (any, error)
}

// ShowOperation retrieves a single resource by ID.
type ShowOperation[ID any] struct {
	Title   string
	Handler func(id ID, trx *ResourceContext) (any, error)
}

// CreateOperation creates a new resource.
type CreateOperation struct {
	Title   string
	Handler func(trx *ResourceContext) (any, error)
}

// UpdateOperation replaces a resource by ID.
type UpdateOperation[ID any] struct {
	Title   string
	Handler func(id ID, trx *ResourceContext) (any, error)
}

// PatchOperation partially updates a resource by ID.
type PatchOperation[ID any] struct {
	Title   string
	Handler func(id ID, trx *ResourceContext) (any, error)
}

// DestroyOperation deletes a resource by ID.
type DestroyOperation[ID any] struct {
	Title   string
	Handler func(id ID, trx *ResourceContext) (any, error)
}

// NewOperation returns the form for creating a new resource.
type NewOperation struct {
	Title   string
	Handler func(trx *ResourceContext) (any, error)
}

// ResourceContext provides the request context for resource operations.
// It wraps the v2 RequestContext and provides helper methods for
// body parsing, parameter access, and response sending.
type ResourceContext struct {
	ReqCtx *v2wf.RequestContext
	// Core is the v1 RequestCoreInterface (may be nil in tests).
	Core requestCore.RequestCoreInterface
}

// GetURLParam returns a URL path parameter by name.
func (c *ResourceContext) GetURLParam(name string) string {
	return c.ReqCtx.Parser.GetURLParam(name)
}

// GetBody binds the request body to the given target.
func (c *ResourceContext) GetBody(target any) error {
	return c.ReqCtx.Parser.GetBody(target)
}

// GetURLQuery binds URL query parameters to the given target.
func (c *ResourceContext) GetURLQuery(target any) error {
	return c.ReqCtx.Parser.GetURLQuery(target)
}

// GetHeaderValue returns the value of the named HTTP request header.
func (c *ResourceContext) GetHeaderValue(name string) string {
	return c.ReqCtx.Parser.GetHeaderValue(name)
}

// SendResponse writes a raw response with the given status, content type, and body.
func (c *ResourceContext) SendResponse(status int, contentType string, body []byte) error {
	type sender interface {
		SendResponse(status int, contentType string, body []byte) error
	}
	if s, ok := c.ReqCtx.Parser.(sender); ok {
		return s.SendResponse(status, contentType, body)
	}
	return nil
}

// Config holds the configuration for registering a resource.
type Config[ID any] struct {
	// Path is the base path for the resource (e.g. "/users").
	Path string

	// Resource implements the seven operations.
	Resource Resource[ID]

	// Core is the v1 RequestCoreInterface for infrastructure access.
	Core requestCore.RequestCoreInterface

	// RespHandler is the v2 response handler.
	RespHandler *v2response.Handler

	// IDParam is the URL parameter name for the resource ID.
	// Default: "id".
	IDParam string

	// IDParser converts a string URL parameter to the ID type.
	// If nil, the string value is used directly for string IDs.
	IDParser func(string) (ID, error)
}

// Register registers all seven resource operations on the given router.
// Operations with nil handlers are skipped.
func Register[ID any](router routing.RouteGroup, config Config[ID]) error {
	idParam := config.IDParam
	if idParam == "" {
		idParam = "id"
	}
	basePath := config.Path

	// Index: GET /{resource}
	if op := config.Resource.Index(); op.Handler != nil {
		h := makeSimpleHandler(op.Title, config, func(ctx *ResourceContext) (any, error) {
			return op.Handler(ctx)
		})
		if err := router.Get(basePath, h); err != nil {
			return err
		}
	}

	// Show: GET /{resource}/{id}
	if op := config.Resource.Show(); op.Handler != nil {
		h := makeSimpleHandler(op.Title, config, func(ctx *ResourceContext) (any, error) {
			id, err := parseID(config, ctx.GetURLParam(idParam))
			if err != nil {
				return nil, err
			}
			return op.Handler(id, ctx)
		})
		if err := router.Get(basePath+"/{"+idParam+"}", h); err != nil {
			return err
		}
	}

	// Create: POST /{resource}
	if op := config.Resource.Create(); op.Handler != nil {
		h := makeSimpleHandler(op.Title, config, func(ctx *ResourceContext) (any, error) {
			return op.Handler(ctx)
		})
		if err := router.Post(basePath, h); err != nil {
			return err
		}
	}

	// Update: PUT /{resource}/{id}
	if op := config.Resource.Update(); op.Handler != nil {
		h := makeSimpleHandler(op.Title, config, func(ctx *ResourceContext) (any, error) {
			id, err := parseID(config, ctx.GetURLParam(idParam))
			if err != nil {
				return nil, err
			}
			return op.Handler(id, ctx)
		})
		if err := router.Put(basePath+"/{"+idParam+"}", h); err != nil {
			return err
		}
	}

	// Patch: PATCH /{resource}/{id}
	if op := config.Resource.Patch(); op.Handler != nil {
		h := makeSimpleHandler(op.Title, config, func(ctx *ResourceContext) (any, error) {
			id, err := parseID(config, ctx.GetURLParam(idParam))
			if err != nil {
				return nil, err
			}
			return op.Handler(id, ctx)
		})
		if err := router.Patch(basePath+"/{"+idParam+"}", h); err != nil {
			return err
		}
	}

	// Destroy: DELETE /{resource}/{id}
	if op := config.Resource.Destroy(); op.Handler != nil {
		h := makeSimpleHandler(op.Title, config, func(ctx *ResourceContext) (any, error) {
			id, err := parseID(config, ctx.GetURLParam(idParam))
			if err != nil {
				return nil, err
			}
			return op.Handler(id, ctx)
		})
		if err := router.Delete(basePath+"/{"+idParam+"}", h); err != nil {
			return err
		}
	}

	// New: GET /{resource}/new
	if op := config.Resource.New(); op.Handler != nil {
		h := makeSimpleHandler(op.Title, config, func(ctx *ResourceContext) (any, error) {
			return op.Handler(ctx)
		})
		if err := router.Get(basePath+"/new", h); err != nil {
			return err
		}
	}

	return nil
}

// makeSimpleHandler creates a routing.Handler from a resource handler function.
// It handles response sending, error handling, and panic recovery.
func makeSimpleHandler[ID any](
	title string,
	config Config[ID],
	fn func(*ResourceContext) (any, error),
) func(ctx *v2wf.RequestContext) error {
	return func(ctx *v2wf.RequestContext) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = nil // suppress error since we handle the response below
				if config.RespHandler != nil {
					if hErr := config.RespHandler.Error(ctx, err); hErr != nil {
						sendRawError(ctx)
					}
				} else {
					sendRawError(ctx)
				}
			}
		}()

		resCtx := &ResourceContext{
			ReqCtx: ctx,
			Core:   config.Core,
		}

		result, err := fn(resCtx)
		if err != nil {
			if config.RespHandler != nil {
				if hErr := config.RespHandler.Error(ctx, err); hErr != nil {
					sendRawError(ctx)
				}
			} else {
				sendRawError(ctx)
			}
			return err
		}

		if config.RespHandler != nil {
			if hErr := config.RespHandler.OK(ctx, result); hErr != nil {
				return hErr
			}
		}
		return nil
	}
}

// sendRawError sends a 500 error response directly to the parser.
func sendRawError(ctx *v2wf.RequestContext) {
	type sender interface {
		SendResponse(status int, contentType string, body []byte) error
	}
	if s, ok := ctx.Parser.(sender); ok {
		_ = s.SendResponse(500, "application/json", []byte(`{"errors":[{"code":"INTERNAL","description":"Internal server error"}]}`))
	}
}

// parseID converts a string URL parameter to the ID type.
func parseID[ID any](config Config[ID], raw string) (ID, error) {
	if config.IDParser != nil {
		return config.IDParser(raw)
	}
	// Default: assume ID is string type
	var zero ID
	if any(zero) == nil || raw == "" {
		return zero, nil
	}
	// Try to cast string to ID (works when ID is string)
	if id, ok := any(raw).(ID); ok {
		return id, nil
	}
	return zero, nil
}

// Suppress unused import warning.
var _ = libRequest.JSON
