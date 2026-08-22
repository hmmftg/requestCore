// Package main is a v2 requestCore example application using chi.
//
// It demonstrates:
//   - Framework-neutral bootstrap with app.Bootstrap
//   - Typed endpoint registration with handlers.RegisterEndpoint
//   - Resource registration with 7 CRUD operations
//   - Session middleware
//   - Background workers
//   - webFramework.AddLog for observability
package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/webFramework"

	"github.com/hmmftg/requestCore/v2/app"
	"github.com/hmmftg/requestCore/v2/handlers"
	"github.com/hmmftg/requestCore/v2/renderers"
	"github.com/hmmftg/requestCore/v2/resources"
	"github.com/hmmftg/requestCore/v2/workers"
)

func main() {
	// Bootstrap the v2 application with chi framework
	application, err := app.Bootstrap(app.Config{
		Framework: app.FrameworkChi,
		Renderer:  renderers.JSONRenderer{},
		WorkerConfig: workers.Config{
			WorkerCount: 4,
			QueueSize:   100,
		},
	})
	if err != nil {
		log.Fatalf("Bootstrap: %v", err)
	}
	defer application.Close()

	// Register a simple health check endpoint
	err = handlers.GetEndpoint[struct{}, HealthResp](
		application.Router,
		nil,
		application.RespHandler,
		"/health",
		func(req *struct{}, trx *handlers.HandlerRequest[struct{}, HealthResp]) (HealthResp, error) {
			webFramework.AddLog(trx.W, "health-check", slog.String("status", "healthy"))
			return HealthResp{Status: "healthy", Time: time.Now().Format(time.RFC3339)}, nil
		},
	)
	if err != nil {
		log.Fatalf("Register health: %v", err)
	}

	// Register a typed endpoint with request body
	err = handlers.PostEndpoint[CreateUserReq, CreateUserResp](
		application.Router,
		nil,
		application.RespHandler,
		"/users",
		func(req *CreateUserReq, trx *handlers.HandlerRequest[CreateUserReq, CreateUserResp]) (CreateUserResp, error) {
			webFramework.AddLog(trx.W, "create-user-req", slog.String("name", req.Name))

			// Submit a background job to send welcome email
			_ = application.Worker.Submit(context.Background(), workers.Job{
				Name: "send-welcome-email",
				Handler: func(ctx *workers.JobContext) error {
					log.Printf("Sending welcome email to %s", req.Name)
					time.Sleep(100 * time.Millisecond)
					return nil
				},
				Options: workers.JobOptions{
					MaxAttempts:    3,
					InitialBackoff: 1 * time.Second,
				},
			})

			return CreateUserResp{ID: "1", Name: req.Name, Created: true}, nil
		},
	)
	if err != nil {
		log.Fatalf("Register create user: %v", err)
	}

	// Register a full CRUD resource
	err = resources.Register[string](application.Router, resources.Config[string]{
		Path:             "/items",
		Resource:         &ItemResource{},
		RespHandler:      application.RespHandler,
		EnablePatchAlias: true,
	})
	if err != nil {
		log.Fatalf("Register items resource: %v", err)
	}

	// Start the server
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on :%s", port)
	log.Printf("Endpoints:")
	log.Printf("  GET  /health       - Health check")
	log.Printf("  POST /users        - Create user")
	log.Printf("  GET  /items        - List items")
	log.Printf("  GET  /items/{id}   - Show item")
	log.Printf("  POST /items        - Create item")
	log.Printf("  PUT  /items/{id}   - Update item")
	log.Printf("  PATCH /items/{id}  - Patch item")
	log.Printf("  DELETE /items/{id} - Delete item")
	log.Printf("  GET  /items/new    - New item form")
	log.Printf("  GET  /items/{id}/edit - Edit item form")

	if err := application.StartWithContext(ctx, ":"+port); err != nil {
		log.Fatalf("Server: %v", err)
	}

	// Coordinated shutdown: stop HTTP server and drain worker pool.
	// application.Close() only drains the worker pool; for full
	// graceful shutdown including the HTTP server, use Shutdown.
	// StartWithContext already calls Shutdown when ctx is cancelled,
	// so Close() here is a safety net for the worker pool.
}

// HealthResp is the response for the health check endpoint.
type HealthResp struct {
	Status string `json:"status"`
	Time   string `json:"time"`
}

// CreateUserReq is the request body for creating a user.
type CreateUserReq struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// CreateUserResp is the response for creating a user.
type CreateUserResp struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Created bool   `json:"created"`
}

// ItemResp is the response for item endpoints.
type ItemResp struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ItemResource implements resources.Resource[string] for items.
type ItemResource struct{}

func (r *ItemResource) List() *handlers.Endpoint {
	return handlers.NewEndpoint[struct{}, []ItemResp]("list-items", libRequest.NoBinding,
		func(req *struct{}, trx *handlers.HandlerRequest[struct{}, []ItemResp]) ([]ItemResp, error) {
			return []ItemResp{
				{ID: "1", Name: "Item 1"},
				{ID: "2", Name: "Item 2"},
			}, nil
		},
	)
}

func (r *ItemResource) Show() *handlers.Endpoint {
	return handlers.NewEndpoint[struct{}, ItemResp]("show-item", libRequest.NoBinding,
		func(req *struct{}, trx *handlers.HandlerRequest[struct{}, ItemResp]) (ItemResp, error) {
			id, err := resources.GetParsedID[string](trx.V2, "id")
			if err != nil {
				return ItemResp{}, err
			}
			return ItemResp{ID: id, Name: "Item " + id}, nil
		},
	)
}

func (r *ItemResource) New() *handlers.Endpoint {
	return handlers.NewEndpoint[struct{}, map[string]any]("new-item", libRequest.NoBinding,
		func(req *struct{}, trx *handlers.HandlerRequest[struct{}, map[string]any]) (map[string]any, error) {
			return map[string]any{"form": "item-form"}, nil
		},
	)
}

func (r *ItemResource) Create() *handlers.Endpoint {
	return handlers.NewEndpoint[ItemResp, ItemResp]("create-item", libRequest.JSON,
		func(req *ItemResp, trx *handlers.HandlerRequest[ItemResp, ItemResp]) (ItemResp, error) {
			return ItemResp{ID: req.ID, Name: req.Name}, nil
		},
	)
}

func (r *ItemResource) Edit() *handlers.Endpoint {
	return handlers.NewEndpoint[struct{}, ItemResp]("edit-item", libRequest.NoBinding,
		func(req *struct{}, trx *handlers.HandlerRequest[struct{}, ItemResp]) (ItemResp, error) {
			id, err := resources.GetParsedID[string](trx.V2, "id")
			if err != nil {
				return ItemResp{}, err
			}
			return ItemResp{ID: id, Name: "Item " + id}, nil
		},
	)
}

func (r *ItemResource) Update() *handlers.Endpoint {
	return handlers.NewEndpoint[ItemResp, ItemResp]("update-item", libRequest.JSON,
		func(req *ItemResp, trx *handlers.HandlerRequest[ItemResp, ItemResp]) (ItemResp, error) {
			id, err := resources.GetParsedID[string](trx.V2, "id")
			if err != nil {
				return ItemResp{}, err
			}
			req.ID = id
			return *req, nil
		},
	)
}

func (r *ItemResource) Destroy() *handlers.Endpoint {
	return handlers.NewEndpoint[struct{}, map[string]any]("delete-item", libRequest.NoBinding,
		func(req *struct{}, trx *handlers.HandlerRequest[struct{}, map[string]any]) (map[string]any, error) {
			id, err := resources.GetParsedID[string](trx.V2, "id")
			if err != nil {
				return nil, err
			}
			return map[string]any{"id": id, "deleted": true}, nil
		},
	)
}

// Suppress unused import warnings.
var _ = renderers.JSONRenderer{}
