// Package main is a v2 requestCore example application using chi.
//
// It demonstrates the generics-first v2 API:
//   - Framework-neutral bootstrap with app.Bootstrap
//   - Typed endpoint registration with handlers.GetEndpoint / PostEndpoint
//   - Lifecycle hooks (WithInitializer, WithFinalizer, WithPersistence)
//   - Resource registration with ResourceBuilder fluent API (7 CRUD operations)
//   - Typed session access with session.GetTyped[T] / SetTyped[T]
//   - Session middleware on a route group
//   - Background workers with webFramework.AddLog observability
package main

import (
	"context"
	"errors"
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
	"github.com/hmmftg/requestCore/v2/routing"
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
		application.Router, nil, application.RespHandler, "/health",
		func(req *struct{}, trx *handlers.HandlerRequest[struct{}, HealthResp]) (HealthResp, error) {
			webFramework.AddLog(trx.W, "health-check", slog.String("status", "healthy"))
			return HealthResp{Status: "healthy", Time: time.Now().Format(time.RFC3339)}, nil
		},
	)
	if err != nil {
		log.Fatalf("Register health: %v", err)
	}

	// --- Typed POST endpoint with lifecycle hooks and background worker ---
	createEndpoint := handlers.NewEndpoint[CreateUserReq, CreateUserResp](
		"create-user", libRequest.JSON,
		func(req *CreateUserReq, trx *handlers.HandlerRequest[CreateUserReq, CreateUserResp]) (CreateUserResp, error) {
			webFramework.AddLog(trx.W, "create-user-req", slog.String("name", req.Name))

			// Submit a background job to send welcome email.
			_ = application.Worker.Submit(context.Background(), workers.Job{
				Name: "send-welcome-email",
				Handler: func(ctx *workers.JobContext) error {
					webFramework.AddLog(ctx.WebFramework, "send-welcome-email-req",
						slog.String("recipient", req.Name))
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
	).
		WithPath("/users").
		WithInitializer(func(trx *handlers.HandlerRequest[CreateUserReq, CreateUserResp]) error {
			// Runs after parsing, before the handler. Compile-time typed.
			webFramework.AddLog(trx.W, "create-user-init", slog.String("status", "initializing"))
			if trx.Request.Name == "" {
				return errors.New("name is required")
			}
			return nil
		}).
		WithFinalizer(func(trx *handlers.HandlerRequest[CreateUserReq, CreateUserResp]) {
			// Always runs, even on panic. Best-effort.
			webFramework.AddLog(trx.W, "create-user-fin",
				slog.String("duration", trx.Duration.String()))
		})

	if err := handlers.RegisterEndpoint(
		application.Router, nil, application.RespHandler,
		"POST", "/users", createEndpoint,
	); err != nil {
		log.Fatalf("Register create user: %v", err)
	}

	// --- CRUD resource via ResourceBuilder fluent API ---
	err = resources.NewResource[string]("/items").
		EnablePatch().
		Register(application.Router, nil, application.RespHandler, &ItemResource{})
	if err != nil {
		log.Fatalf("Register items resource: %v", err)
	}

	// --- Typed session access on a protected route group ---
	// The session middleware returns a function with the same signature as
	// routing.Middleware; we wrap it to satisfy the routing.Middleware type.
	sessionMw := routing.Middleware(func(next routing.Handler) routing.Handler {
		return session.Middleware(application.Sessions, "session")(next)
	})
	api := application.Register("/api", sessionMw)

	err = handlers.GetEndpoint[struct{}, ProfileResp](
		api, nil, application.RespHandler, "/profile",
		func(req *struct{}, trx *handlers.HandlerRequest[struct{}, ProfileResp]) (ProfileResp, error) {
			// Typed session access — no runtime type assertions.
			if trx.V2.Session == nil {
				return ProfileResp{}, errors.New("no session")
			}

			sess, ok := trx.V2.Session.(*session.Session)
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

			webFramework.AddLog(trx.W, "profile-req", slog.Int("visits", visits))

			return ProfileResp{Name: "guest", Visits: visits}, nil
		},
	)
	if err != nil {
		log.Fatalf("Register profile: %v", err)
	}

	// --- Typed POST endpoint ---
	err = handlers.PostEndpoint[EchoReq, EchoResp](
		application.Router, nil, application.RespHandler, "/echo",
		func(req *EchoReq, trx *handlers.HandlerRequest[EchoReq, EchoResp]) (EchoResp, error) {
			webFramework.AddLog(trx.W, "echo-req", slog.String("message", req.Message))
			// The framework renders the response automatically via the
			// configured renderer. For explicit typed rendering outside
			// the endpoint lifecycle, use OKTyped:
			//   application.RespHandler.OKTyped(trx.V2, EchoResp{...})
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
	log.Printf("  POST /users           - Create user (lifecycle hooks + worker)")
	log.Printf("  GET  /items           - List items (resource)")
	log.Printf("  GET  /items/{id}      - Show item")
	log.Printf("  POST /items           - Create item (WithPersistence)")
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
	return handlers.NewEndpoint[struct{}, []ItemResp]("list-items", libRequest.NoBinding,
		func(req *struct{}, trx *handlers.HandlerRequest[struct{}, []ItemResp]) ([]ItemResp, error) {
			return []ItemResp{
				{ID: "1", Name: "Item 1"},
				{ID: "2", Name: "Item 2"},
			}, nil
		},
	)
}

func (r *ItemResource) Show() handlers.EndpointRuntime {
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

func (r *ItemResource) New() handlers.EndpointRuntime {
	return handlers.NewEndpoint[struct{}, map[string]any]("new-item", libRequest.NoBinding,
		func(req *struct{}, trx *handlers.HandlerRequest[struct{}, map[string]any]) (map[string]any, error) {
			return map[string]any{"form": "item-form"}, nil
		},
	)
}

// Create demonstrates WithPersistence on a resource endpoint.
func (r *ItemResource) Create() handlers.EndpointRuntime {
	return handlers.NewEndpoint[ItemResp, ItemResp]("create-item", libRequest.JSON,
		func(req *ItemResp, trx *handlers.HandlerRequest[ItemResp, ItemResp]) (ItemResp, error) {
			webFramework.AddLog(trx.W, "create-item-req", slog.String("name", req.Name))
			return ItemResp{ID: req.ID, Name: req.Name}, nil
		},
	).WithPersistence(handlers.NewPersister[ItemResp, ItemResp](
		func(path string, trx *handlers.HandlerRequest[ItemResp, ItemResp]) error {
			webFramework.AddLog(trx.W, "create-item-insert", slog.String("path", path))
			return nil
		},
		func(path string, trx *handlers.HandlerRequest[ItemResp, ItemResp]) error {
			webFramework.AddLog(trx.W, "create-item-update", slog.String("path", path))
			return nil
		},
	))
}

func (r *ItemResource) Edit() handlers.EndpointRuntime {
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

func (r *ItemResource) Update() handlers.EndpointRuntime {
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

func (r *ItemResource) Destroy() handlers.EndpointRuntime {
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
