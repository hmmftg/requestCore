// Package main is a v2 requestCore example application using chi.
//
// It demonstrates the canonical v2 API:
//   - Framework-neutral bootstrap with app.Bootstrap
//   - Typed endpoint registration with handlers.Get / Post / RegisterEndpoint
//   - Resource registration with ResourceBuilder fluent API (7 CRUD operations)
//   - Typed session access with session.GetTyped[T] / SetTyped[T]
//   - Session middleware on a route group
//   - Background workers
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hmmftg/requestCore/v2/app"
	"github.com/hmmftg/requestCore/v2/handlers"
	"github.com/hmmftg/requestCore/v2/renderers"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/resources"
	"github.com/hmmftg/requestCore/v2/session"
	"github.com/hmmftg/requestCore/v2/workers"
)

func main() {
	// Bootstrap the v2 application with chi framework and session support.
	store, err := session.NewCookieStore(session.CookieStoreConfig{
		SecretKey: []byte("example-secret-key-32-bytes-long!!"),
	})
	if err != nil {
		log.Fatalf("CookieStore: %v", err)
	}

	application, err := app.Bootstrap(app.Config{
		Framework:     app.FrameworkChi,
		Renderer:      renderers.JSONRenderer{},
		SessionStore:  store,
		SessionSecret: "example-signing-secret",
		WorkerConfig:  workers.Config{WorkerCount: 4, QueueSize: 100},
	})
	if err != nil {
		log.Fatalf("Bootstrap: %v", err)
	}
	defer application.Close()

	// --- Typed GET endpoint ---
	err = handlers.GetEndpoint[struct{}, HealthResp](
		application.Router, application.Executor, "/health",
		func(ctx *request.Context, req struct{}) (HealthResp, error) {
			return HealthResp{Status: "healthy", Time: time.Now().Format(time.RFC3339)}, nil
		},
	)
	if err != nil {
		log.Fatalf("Register health: %v", err)
	}

	// --- Typed POST endpoint with background worker ---
	createEndpoint := handlers.Post[CreateUserReq, CreateUserResp](
		"create-user", "/users",
		func(ctx *request.Context, req CreateUserReq) (CreateUserResp, error) {
			// Submit a background job to send welcome email.
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

	if err := handlers.RegisterEndpoint(
		application.Router, application.Executor, createEndpoint,
	); err != nil {
		log.Fatalf("Register create user: %v", err)
	}

	// --- CRUD resource via ResourceBuilder fluent API ---
	err = resources.NewResource[string]("/items").
		EnablePatch().
		Register(application.Router, application.Executor, &ItemResource{})
	if err != nil {
		log.Fatalf("Register items resource: %v", err)
	}

	// --- Typed session access on a protected route group ---
	// Use the app's SessionMiddleware which is compatible with the new
	// routing contract.
	sessionMw := app.SessionMiddleware(application.Sessions, "session")
	api := application.Register("/api", sessionMw)

	err = handlers.GetEndpoint[struct{}, ProfileResp](
		api, application.Executor, "/profile",
		func(ctx *request.Context, req struct{}) (ProfileResp, error) {
			// Typed session access — no runtime type assertions.
			principal := ctx.Principal()
			if principal == nil {
				return ProfileResp{}, errors.New("no session")
			}

			sess, ok := principal.(*session.Session)
			if !ok {
				return ProfileResp{}, errors.New("session type assertion failed")
			}

			// Store a typed value.
			session.SetTyped(sess, "visits", 1)

			// Retrieve a typed value with compile-time type checking.
			visits, vErr := session.GetTyped[int](sess, "visits")
			if vErr != nil {
				visits = 0
			}

			return ProfileResp{Name: "guest", Visits: visits}, nil
		},
	)
	if err != nil {
		log.Fatalf("Register profile: %v", err)
	}

	// --- Typed POST endpoint ---
	err = handlers.PostEndpoint[EchoReq, EchoResp](
		application.Router, application.Executor, "/echo",
		func(ctx *request.Context, req EchoReq) (EchoResp, error) {
			return EchoResp{Message: req.Message}, nil
		},
	)
	if err != nil {
		log.Fatalf("Register echo: %v", err)
	}

	// Start the server.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on :%s", port)
	log.Printf("Endpoints:")
	log.Printf("  GET  /health          - Health check (typed GET)")
	log.Printf("  POST /users           - Create user (worker)")
	log.Printf("  GET  /items           - List items (resource)")
	log.Printf("  GET  /items/{id}      - Show item")
	log.Printf("  POST /items           - Create item")
	log.Printf("  PUT  /items/{id}      - Update item")
	log.Printf("  PATCH /items/{id}     - Patch item (alias)")
	log.Printf("  DELETE /items/{id}    - Delete item")
	log.Printf("  GET  /items/new       - New item form")
	log.Printf("  GET  /items/{id}/edit - Edit item form")
	log.Printf("  GET  /api/profile     - Profile (typed session access)")
	log.Printf("  POST /echo            - Echo (typed POST)")

	if err := application.StartWithContext(ctx, ":"+port); err != nil {
		log.Fatalf("Server: %v", err)
	}
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

// EchoReq is the request body for the echo endpoint.
type EchoReq struct {
	Message string `json:"message"`
}

// EchoResp is the response for the echo endpoint.
type EchoResp struct {
	Message string `json:"message"`
}

// ProfileResp is the response for the profile endpoint.
type ProfileResp struct {
	Name   string `json:"name"`
	Visits int    `json:"visits"`
}

// ItemResp is the response for item endpoints.
type ItemResp struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ItemResource implements resources.Resource[string] for items.
type ItemResource struct{}

func (r *ItemResource) List() handlers.EndpointRuntime {
	return handlers.Get[struct{}, []ItemResp]("list-items", "/items",
		func(ctx *request.Context, req struct{}) ([]ItemResp, error) {
			return []ItemResp{
				{ID: "1", Name: "Item 1"},
				{ID: "2", Name: "Item 2"},
			}, nil
		},
	)
}

func (r *ItemResource) Show() handlers.EndpointRuntime {
	return handlers.Get[struct{}, ItemResp]("show-item", "/items/{id}",
		func(ctx *request.Context, req struct{}) (ItemResp, error) {
			id, err := resources.GetParsedID[string](ctx, "id")
			if err != nil {
				return ItemResp{}, err
			}
			return ItemResp{ID: id, Name: "Item " + id}, nil
		},
	)
}

func (r *ItemResource) New() handlers.EndpointRuntime {
	return handlers.Get[struct{}, map[string]any]("new-item", "/items/new",
		func(ctx *request.Context, req struct{}) (map[string]any, error) {
			return map[string]any{"form": "item-form"}, nil
		},
	)
}

func (r *ItemResource) Create() handlers.EndpointRuntime {
	return handlers.Post[ItemResp, ItemResp]("create-item", "/items",
		func(ctx *request.Context, req ItemResp) (ItemResp, error) {
			return ItemResp{ID: req.ID, Name: req.Name}, nil
		},
	)
}

func (r *ItemResource) Edit() handlers.EndpointRuntime {
	return handlers.Get[struct{}, ItemResp]("edit-item", "/items/{id}/edit",
		func(ctx *request.Context, req struct{}) (ItemResp, error) {
			id, err := resources.GetParsedID[string](ctx, "id")
			if err != nil {
				return ItemResp{}, err
			}
			return ItemResp{ID: id, Name: "Item " + id}, nil
		},
	)
}

func (r *ItemResource) Update() handlers.EndpointRuntime {
	return handlers.Put[ItemResp, ItemResp]("update-item", "/items/{id}",
		func(ctx *request.Context, req ItemResp) (ItemResp, error) {
			id, err := resources.GetParsedID[string](ctx, "id")
			if err != nil {
				return ItemResp{}, err
			}
			req.ID = id
			return req, nil
		},
	)
}

func (r *ItemResource) Destroy() handlers.EndpointRuntime {
	return handlers.Delete[struct{}, map[string]any]("destroy-item", "/items/{id}",
		func(ctx *request.Context, req struct{}) (map[string]any, error) {
			id, err := resources.GetParsedID[string](ctx, "id")
			if err != nil {
				return nil, err
			}
			return map[string]any{"id": id, "deleted": true}, nil
		},
	)
}

// Suppress unused import warnings.
var _ = renderers.JSONRenderer{}
