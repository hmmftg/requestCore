package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// generateHandler generates a new handler file.
func generateHandler(name string) error {
	pascalName := toPascalCase(name)
	camelName := toCamelCase(name)

	template := `// Package handlers contains the {PASCAL} handler.
package handlers

import (
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/v2/handlers"
)

// {PASCAL}Req is the request body for the {NAME} handler.
type {PASCAL}Req struct {
	// Add request fields here
}

// {PASCAL}Resp is the response body for the {NAME} handler.
type {PASCAL}Resp struct {
	// Add response fields here
}

// {PASCAL}Handler implements handlers.HandlerInterface for the {NAME} endpoint.
type {PASCAL}Handler struct{}

// Parameters returns the handler configuration.
func (h *{PASCAL}Handler) Parameters() handlers.HandlerParameters[{PASCAL}Req, {PASCAL}Resp] {
	return handlers.HandlerParameters[{PASCAL}Req, {PASCAL}Resp]{
		Title: "{NAME}",
		Path:  "/{NAME}",
		Body:  libRequest.JSON,
	}
}

// Initializer runs after validating the request.
func (h *{PASCAL}Handler) Initializer(req *handlers.HandlerRequest[{PASCAL}Req, {PASCAL}Resp]) error {
	return nil
}

// Handler is the main handler logic.
func (h *{PASCAL}Handler) Handler(req *handlers.HandlerRequest[{PASCAL}Req, {PASCAL}Resp]) ({PASCAL}Resp, error) {
	return {PASCAL}Resp{}, nil
}

// Finalizer runs after sending back the response.
func (h *{PASCAL}Handler) Finalizer(req *handlers.HandlerRequest[{PASCAL}Req, {PASCAL}Resp]) {}

// Simulation handles simulation mode.
func (h *{PASCAL}Handler) Simulation(req *handlers.HandlerRequest[{PASCAL}Req, {PASCAL}Resp]) ({PASCAL}Resp, error) {
	return {PASCAL}Resp{}, nil
}
`

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

// generateResource generates a new resource file with 7 operations.
func generateResource(name string) error {
	pascalName := toPascalCase(name)
	camelName := toCamelCase(name)

	template := `// Package resources contains the {NAME} resource.
package resources

import (
	"github.com/hmmftg/requestCore/v2/resources"
)

// {PASCAL}Resource implements resources.Resource[string] for the {NAME} resource.
type {PASCAL}Resource struct{}

// Index returns a list of {NAME}.
func (r *{PASCAL}Resource) Index() resources.IndexOperation {
	return resources.IndexOperation{
		Title: "list-{NAME}",
		Handler: func(trx *resources.ResourceContext) (any, error) {
			return []map[string]any{}, nil
		},
	}
}

// Show returns a single {NAME} by ID.
func (r *{PASCAL}Resource) Show() resources.ShowOperation[string] {
	return resources.ShowOperation[string]{
		Title: "show-{NAME}",
		Handler: func(id string, trx *resources.ResourceContext) (any, error) {
			return map[string]any{"id": id}, nil
		},
	}
}

// Create creates a new {NAME}.
func (r *{PASCAL}Resource) Create() resources.CreateOperation {
	return resources.CreateOperation{
		Title: "create-{NAME}",
		Handler: func(trx *resources.ResourceContext) (any, error) {
			return map[string]any{"created": true}, nil
		},
	}
}

// Update replaces a {NAME} by ID.
func (r *{PASCAL}Resource) Update() resources.UpdateOperation[string] {
	return resources.UpdateOperation[string]{
		Title: "update-{NAME}",
		Handler: func(id string, trx *resources.ResourceContext) (any, error) {
			return map[string]any{"id": id, "updated": true}, nil
		},
	}
}

// Patch partially updates a {NAME} by ID.
func (r *{PASCAL}Resource) Patch() resources.PatchOperation[string] {
	return resources.PatchOperation[string]{
		Title: "patch-{NAME}",
		Handler: func(id string, trx *resources.ResourceContext) (any, error) {
			return map[string]any{"id": id, "patched": true}, nil
		},
	}
}

// Destroy deletes a {NAME} by ID.
func (r *{PASCAL}Resource) Destroy() resources.DestroyOperation[string] {
	return resources.DestroyOperation[string]{
		Title: "delete-{NAME}",
		Handler: func(id string, trx *resources.ResourceContext) (any, error) {
			return map[string]any{"id": id, "deleted": true}, nil
		},
	}
}

// New returns the form for creating a new {NAME}.
func (r *{PASCAL}Resource) New() resources.NewOperation {
	return resources.NewOperation{
		Title: "new-{NAME}",
		Handler: func(trx *resources.ResourceContext) (any, error) {
			return map[string]any{"form": "new-{NAME}"}, nil
		},
	}
}
`

	content := strings.NewReplacer(
		"{PASCAL}", pascalName,
		"{NAME}", name,
		"{CAMEL}", camelName,
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
}
`

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
}
`

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
