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
	"log/slog"

	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/v2/handlers"
	"github.com/hmmftg/requestCore/webFramework"
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
//	    router, core, respHandler, "/{NAME}",
//	    {PASCAL}Handler,
//	)
//
// or for a GET endpoint:
//
//	handlers.GetEndpoint[{PASCAL}Req, {PASCAL}Resp](
//	    router, core, respHandler, "/{NAME}",
//	    {PASCAL}Handler,
//	)
func {PASCAL}Handler(req *{PASCAL}Req, trx *handlers.HandlerRequest[{PASCAL}Req, {PASCAL}Resp]) ({PASCAL}Resp, error) {
	// Log to the Splunk transaction pipeline via webFramework.AddLog.
	webFramework.AddLog(trx.W, "{NAME}-req", slog.String("status", "processing"))

	// TODO: implement handler logic.

	return {PASCAL}Resp{}, nil
}

// {PASCAL}Endpoint returns a typed Endpoint for the {NAME} handler.
// Use this with handlers.RegisterEndpoint for custom HTTP methods or
// when you need to attach lifecycle hooks.
func {PASCAL}Endpoint() *handlers.Endpoint {
	return handlers.NewEndpoint[{PASCAL}Req, {PASCAL}Resp](
		"{NAME}",
		libRequest.JSON,
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
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/v2/handlers"
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
func (r *{PASCAL}Resource) List() *handlers.Endpoint {
	return handlers.NewEndpoint[{PASCAL}ListReq, {PASCAL}ListResp](
		"list-{NAME}",
		libRequest.NoBinding,
		func(req *{PASCAL}ListReq, trx *handlers.HandlerRequest[{PASCAL}ListReq, {PASCAL}ListResp]) ({PASCAL}ListResp, error) {
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
func (r *{PASCAL}Resource) Show() *handlers.Endpoint {
	return handlers.NewEndpoint[{PASCAL}ShowReq, {PASCAL}ShowResp](
		"show-{NAME}",
		libRequest.NoBinding,
		func(req *{PASCAL}ShowReq, trx *handlers.HandlerRequest[{PASCAL}ShowReq, {PASCAL}ShowResp]) ({PASCAL}ShowResp, error) {
			id, err := resources.GetParsedID[string](trx.V2, "id")
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
func (r *{PASCAL}Resource) New() *handlers.Endpoint {
	return handlers.NewEndpoint[{PASCAL}NewReq, {PASCAL}NewResp](
		"new-{NAME}",
		libRequest.NoBinding,
		func(req *{PASCAL}NewReq, trx *handlers.HandlerRequest[{PASCAL}NewReq, {PASCAL}NewResp]) ({PASCAL}NewResp, error) {
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
func (r *{PASCAL}Resource) Create() *handlers.Endpoint {
	return handlers.NewEndpoint[{PASCAL}CreateReq, {PASCAL}CreateResp](
		"create-{NAME}",
		libRequest.JSON,
		func(req *{PASCAL}CreateReq, trx *handlers.HandlerRequest[{PASCAL}CreateReq, {PASCAL}CreateResp]) ({PASCAL}CreateResp, error) {
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
func (r *{PASCAL}Resource) Edit() *handlers.Endpoint {
	return handlers.NewEndpoint[{PASCAL}EditReq, {PASCAL}EditResp](
		"edit-{NAME}",
		libRequest.NoBinding,
		func(req *{PASCAL}EditReq, trx *handlers.HandlerRequest[{PASCAL}EditReq, {PASCAL}EditResp]) ({PASCAL}EditResp, error) {
			id, err := resources.GetParsedID[string](trx.V2, "id")
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
func (r *{PASCAL}Resource) Update() *handlers.Endpoint {
	return handlers.NewEndpoint[{PASCAL}UpdateReq, {PASCAL}UpdateResp](
		"update-{NAME}",
		libRequest.JSON,
		func(req *{PASCAL}UpdateReq, trx *handlers.HandlerRequest[{PASCAL}UpdateReq, {PASCAL}UpdateResp]) ({PASCAL}UpdateResp, error) {
			id, err := resources.GetParsedID[string](trx.V2, "id")
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
func (r *{PASCAL}Resource) Destroy() *handlers.Endpoint {
	return handlers.NewEndpoint[{PASCAL}DestroyReq, {PASCAL}DestroyResp](
		"delete-{NAME}",
		libRequest.NoBinding,
		func(req *{PASCAL}DestroyReq, trx *handlers.HandlerRequest[{PASCAL}DestroyReq, {PASCAL}DestroyResp]) ({PASCAL}DestroyResp, error) {
			id, err := resources.GetParsedID[string](trx.V2, "id")
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
	"github.com/hmmftg/requestCore/v2/routing"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// {PASCAL}Middleware is a v2 middleware that [describe what it does].
func {PASCAL}Middleware() routing.Middleware {
	return func(next routing.Handler) routing.Handler {
		return func(ctx *v2wf.RequestContext) error {
			// Pre-processing: runs before the handler

			err := next(ctx)

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
	goModContent := fmt.Sprintf("module %s\n\ngo 1.25.5\n\nrequire github.com/hmmftg/requestCore/v2 v2.0.0-alpha\n", name)

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
