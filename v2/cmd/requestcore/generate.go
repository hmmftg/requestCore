package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// generateHandler generates a new handler file with typed request/response
// types and a handler function compatible with handlers.GetEndpoint /
// handlers.PostEndpoint registration.
func generateHandler(name string) error {
	pascalName := toPascalCase(name)
	camelName := toCamelCase(name)

	template := `// Package handlers contains the {PASCAL} handler.
package handlers

import (
	"github.com/hmmftg/requestCore/v2/handlers"
	"github.com/hmmftg/requestCore/v2/request"
)

// {PASCAL}Req is the request body for the {NAME} handler.
type {PASCAL}Req struct {
	// Add request fields here
}

// {PASCAL}Resp is the response body for the {NAME} handler.
type {PASCAL}Resp struct {
	// Add response fields here
}

// {PASCAL}Handler is the handler function for the {NAME} endpoint.
// Register it via:
//
//	handlers.PostEndpoint[{PASCAL}Req, {PASCAL}Resp](
//	    router, exec, "/{NAME}",
//	    {PASCAL}Handler,
//	)
//
// or for a GET endpoint:
//
//	handlers.GetEndpoint[{PASCAL}Req, {PASCAL}Resp](
//	    router, exec, "/{NAME}",
//	    {PASCAL}Handler,
//	)
func {PASCAL}Handler(ctx *request.Context, req {PASCAL}Req) ({PASCAL}Resp, error) {
	// TODO: implement handler logic.

	return {PASCAL}Resp{}, nil
}

// {PASCAL}Endpoint returns a typed Endpoint for the {NAME} handler.
// Use this with handlers.RegisterEndpoint for custom HTTP methods or
// when you need advanced configuration via the Inner() method.
func {PASCAL}Endpoint() *handlers.Endpoint[{PASCAL}Req, {PASCAL}Resp] {
	return handlers.Post[{PASCAL}Req, {PASCAL}Resp](
		"{NAME}",
		"/{NAME}",
		{PASCAL}Handler,
	)
}`

	content := strings.NewReplacer(
		"{PASCAL}", pascalName,
		"{NAME}", name,
		"{CAMEL}", camelName,
	).Replace(template)

	dir := "handlers"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create handlers dir: %w", err)
	}

	path := filepath.Join(dir, camelName+".go")
	if err := writeFile(path, content); err != nil {
		return fmt.Errorf("write handler file: %w", err)
	}

	fmt.Printf("Generated handler: %s\n", path)
	return nil
}

// generateResource generates a new resource file with 7 CRUD operations
// matching the resources.Resource[ID] interface. The __BT__ placeholder
// is replaced with a backtick after the raw-string template is processed,
// because Go raw string literals cannot contain backticks.
func generateResource(name string) error {
	pascalName := toPascalCase(name)
	camelName := toCamelCase(name)

	template := `// Package resources contains the {NAME} resource.
package resources

import (
	"github.com/hmmftg/requestCore/v2/handlers"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/resources"
)

// {PASCAL}Resource implements resources.Resource[string] for the {NAME} resource.
type {PASCAL}Resource struct{}

// {PASCAL}ListReq is the request body for listing {NAME}.
type {PASCAL}ListReq struct{}

// {PASCAL}ListResp is the response body for listing {NAME}.
type {PASCAL}ListResp struct {
	Items []map[string]any __BT__json:"items"__BT__
}

// List returns a list of {NAME}.
func (r *{PASCAL}Resource) List() handlers.EndpointRuntime {
	return handlers.Get[{PASCAL}ListReq, {PASCAL}ListResp](
		"list-{NAME}",
		"/{NAME}",
		func(ctx *request.Context, req {PASCAL}ListReq) ({PASCAL}ListResp, error) {
			return {PASCAL}ListResp{Items: []map[string]any{}}, nil
		},
	)
}

// {PASCAL}ShowReq is the request body for showing a {NAME}.
type {PASCAL}ShowReq struct{}

// {PASCAL}ShowResp is the response body for showing a {NAME}.
type {PASCAL}ShowResp struct {
	ID string __BT__json:"id"__BT__
}

// Show returns a single {NAME} by ID.
func (r *{PASCAL}Resource) Show() handlers.EndpointRuntime {
	return handlers.Get[{PASCAL}ShowReq, {PASCAL}ShowResp](
		"show-{NAME}",
		"/{NAME}/{id}",
		func(ctx *request.Context, req {PASCAL}ShowReq) ({PASCAL}ShowResp, error) {
			id, err := resources.GetParsedID[string](ctx, "id")
			if err != nil {
				return {PASCAL}ShowResp{}, err
			}
			return {PASCAL}ShowResp{ID: id}, nil
		},
	)
}

// {PASCAL}NewReq is the request body for the new {NAME} form.
type {PASCAL}NewReq struct{}

// {PASCAL}NewResp is the response body for the new {NAME} form.
type {PASCAL}NewResp struct {
	Form string __BT__json:"form"__BT__
}

// New returns the form for creating a new {NAME}.
func (r *{PASCAL}Resource) New() handlers.EndpointRuntime {
	return handlers.Get[{PASCAL}NewReq, {PASCAL}NewResp](
		"new-{NAME}",
		"/{NAME}/new",
		func(ctx *request.Context, req {PASCAL}NewReq) ({PASCAL}NewResp, error) {
			return {PASCAL}NewResp{Form: "new-{NAME}"}, nil
		},
	)
}

// {PASCAL}CreateReq is the request body for creating a {NAME}.
type {PASCAL}CreateReq struct {
	// Add create fields here
}

// {PASCAL}CreateResp is the response body for creating a {NAME}.
type {PASCAL}CreateResp struct {
	Created bool __BT__json:"created"__BT__
}

// Create creates a new {NAME}.
func (r *{PASCAL}Resource) Create() handlers.EndpointRuntime {
	return handlers.Post[{PASCAL}CreateReq, {PASCAL}CreateResp](
		"create-{NAME}",
		"/{NAME}",
		func(ctx *request.Context, req {PASCAL}CreateReq) ({PASCAL}CreateResp, error) {
			return {PASCAL}CreateResp{Created: true}, nil
		},
	)
}

// {PASCAL}EditReq is the request body for editing a {NAME}.
type {PASCAL}EditReq struct{}

// {PASCAL}EditResp is the response body for editing a {NAME}.
type {PASCAL}EditResp struct {
	ID string __BT__json:"id"__BT__
}

// Edit returns the form for editing a {NAME} by ID.
func (r *{PASCAL}Resource) Edit() handlers.EndpointRuntime {
	return handlers.Get[{PASCAL}EditReq, {PASCAL}EditResp](
		"edit-{NAME}",
		"/{NAME}/{id}/edit",
		func(ctx *request.Context, req {PASCAL}EditReq) ({PASCAL}EditResp, error) {
			id, err := resources.GetParsedID[string](ctx, "id")
			if err != nil {
				return {PASCAL}EditResp{}, err
			}
			return {PASCAL}EditResp{ID: id}, nil
		},
	)
}

// {PASCAL}UpdateReq is the request body for updating a {NAME}.
type {PASCAL}UpdateReq struct {
	// Add update fields here
}

// {PASCAL}UpdateResp is the response body for updating a {NAME}.
type {PASCAL}UpdateResp struct {
	ID      string __BT__json:"id"__BT__
	Updated bool   __BT__json:"updated"__BT__
}

// Update replaces a {NAME} by ID.
func (r *{PASCAL}Resource) Update() handlers.EndpointRuntime {
	return handlers.Put[{PASCAL}UpdateReq, {PASCAL}UpdateResp](
		"update-{NAME}",
		"/{NAME}/{id}",
		func(ctx *request.Context, req {PASCAL}UpdateReq) ({PASCAL}UpdateResp, error) {
			id, err := resources.GetParsedID[string](ctx, "id")
			if err != nil {
				return {PASCAL}UpdateResp{}, err
			}
			return {PASCAL}UpdateResp{ID: id, Updated: true}, nil
		},
	)
}

// {PASCAL}DestroyReq is the request body for deleting a {NAME}.
type {PASCAL}DestroyReq struct{}

// {PASCAL}DestroyResp is the response body for deleting a {NAME}.
type {PASCAL}DestroyResp struct {
	ID      string __BT__json:"id"__BT__
	Deleted bool   __BT__json:"deleted"__BT__
}

// Destroy deletes a {NAME} by ID.
func (r *{PASCAL}Resource) Destroy() handlers.EndpointRuntime {
	return handlers.Delete[{PASCAL}DestroyReq, {PASCAL}DestroyResp](
		"delete-{NAME}",
		"/{NAME}/{id}",
		func(ctx *request.Context, req {PASCAL}DestroyReq) ({PASCAL}DestroyResp, error) {
			id, err := resources.GetParsedID[string](ctx, "id")
			if err != nil {
				return {PASCAL}DestroyResp{}, err
			}
			return {PASCAL}DestroyResp{ID: id, Deleted: true}, nil
		},
	)
}`

	content := strings.NewReplacer(
		"{PASCAL}", pascalName,
		"{NAME}", name,
		"{CAMEL}", camelName,
		"__BT__", "`",
	).Replace(template)

	dir := "resources"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create resources dir: %w", err)
	}

	path := filepath.Join(dir, camelName+".go")
	if err := writeFile(path, content); err != nil {
		return fmt.Errorf("write resource file: %w", err)
	}

	fmt.Printf("Generated resource: %s\n", path)
	return nil
}

// generateMiddleware generates a new middleware file.
func generateMiddleware(name string) error {
	pascalName := toPascalCase(name)
	camelName := toCamelCase(name)

	template := `// Package middleware contains the {NAME} middleware.
package middleware

import (
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/routing"
)

// {PASCAL}Middleware is a v2 middleware that [describe what it does].
func {PASCAL}Middleware() routing.Middleware {
	return func(next routing.Handler) routing.Handler {
		return func(ctx *request.Context, transport routing.Transport) error {
			// Pre-processing: runs before the handler

			err := next(ctx, transport)

			// Post-processing: runs after the handler

			return err
		}
	}
}`

	content := strings.NewReplacer(
		"{PASCAL}", pascalName,
		"{NAME}", name,
		"{CAMEL}", camelName,
	).Replace(template)

	dir := "middleware"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create middleware dir: %w", err)
	}

	path := filepath.Join(dir, camelName+".go")
	if err := writeFile(path, content); err != nil {
		return fmt.Errorf("write middleware file: %w", err)
	}

	fmt.Printf("Generated middleware: %s\n", path)
	return nil
}

// generateProject generates a new v2 project structure.
func generateProject(name string) error {
	dirs := []string{
		name,
		filepath.Join(name, "cmd", name),
		filepath.Join(name, "handlers"),
		filepath.Join(name, "resources"),
		filepath.Join(name, "middleware"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}

	// main.go
	mainTemplate := `package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hmmftg/requestCore/v2/app"
)

func main() {
	application, err := app.Bootstrap(app.Config{
		Framework: app.FrameworkChi,
	})
	if err != nil {
		log.Fatalf("Bootstrap: %v", err)
	}
	defer application.Close()

	// Register routes here
	// application.Router.Get("/health", ...)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Println("Starting server on :8080")
	if err := application.StartWithContext(ctx, ":8080"); err != nil {
		log.Fatalf("Server: %v", err)
	}
	_ = os.Stdout
}`

	if err := writeFile(filepath.Join(name, "cmd", name, "main.go"), mainTemplate); err != nil {
		return fmt.Errorf("write main.go: %w", err)
	}

	// go.mod
	goModContent := fmt.Sprintf("module %s\n\ngo 1.27.0\n\nrequire github.com/hmmftg/requestCore/v2 %s\n", name, Version)

	if err := writeFile(filepath.Join(name, "go.mod"), goModContent); err != nil {
		return fmt.Errorf("write go.mod: %w", err)
	}

	// README.md
	readmeContent := "# " + name + "\n\n" +
		"A v2 requestCore application.\n\n" +
		"## Getting Started\n\n" +
		"1. Install dependencies:\n" +
		"   ```bash\n" +
		"   go mod tidy\n" +
		"   ```\n\n" +
		"2. Run the server:\n" +
		"   ```bash\n" +
		"   go run cmd/" + name + "/main.go\n" +
		"   ```\n\n" +
		"3. The server starts on http://localhost:8080\n\n" +
		"## Project Structure\n\n" +
		"- `cmd/" + name + "/` - Application entry point\n" +
		"- `handlers/` - Request handlers\n" +
		"- `resources/` - Resource definitions (7 CRUD operations)\n" +
		"- `middleware/` - Custom middleware\n"

	if err := writeFile(filepath.Join(name, "README.md"), readmeContent); err != nil {
		return fmt.Errorf("write README.md: %w", err)
	}

	fmt.Printf("Generated project: %s\n", name)
	return nil
}
