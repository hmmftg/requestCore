package libNetHttp

import (
	"context"
	"log"
	"net/http"

	"github.com/hmmftg/requestCore/webFramework"
)

type ContextInitiator interface {
	InitContext(r *http.Request, w http.ResponseWriter) webFramework.WebFramework
	Respond(int, int, string, any, bool, webFramework.WebFramework)
}

func NetHttpErrorHandler(path, title string, handler ContextInitiator) http.HandlerFunc {
	log.Println("ErrorHandler: ", path, title)
	return func(w http.ResponseWriter, r *http.Request) {
		// Create a custom error context
		netHttpCtx := InitContext(r, w)
		wf := webFramework.WebFramework{
			Parser: netHttpCtx,
		}

		// Handle different types of errors
		// For net/http, we don't have built-in error types like Fiber/Gin
		// We'll handle common HTTP errors

		// Check for 404 (Not Found)
		if r.URL.Path != path {
			handler.Respond(http.StatusNotFound, 1, "PAGE_NOT_FOUND", "Page not found", true, wf)
			return
		}

		// Check for method not allowed
		if r.Method != "GET" && r.Method != "POST" && r.Method != "PUT" && r.Method != "DELETE" && r.Method != "PATCH" {
			handler.Respond(http.StatusMethodNotAllowed, 1, "METHOD_NOT_ALLOWED", "Method not allowed", true, wf)
			return
		}

		// Generic internal server error
		handler.Respond(http.StatusInternalServerError, 1, "INTERNAL_ERROR", "Internal server error", true, wf)
	}
}

// NetHttpHandler wraps a handler function to work with net/http
func NetHttpHandler(handler any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Convert the handler to work with net/http context
		// The handler expects a context.Context, so we pass the request context
		if handlerFunc, ok := handler.(func(context.Context)); ok {
			handlerFunc(r.Context())
		} else {
			// If it's not the expected type, log an error
			log.Printf("Invalid handler type: %T", handler)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// Middleware function type for net/http
type Middleware func(http.Handler) http.Handler

// ChainMiddleware chains multiple middleware functions
func ChainMiddleware(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// CORS middleware for net/http
func CORSMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set CORS headers
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, User-Id, Program, Module, Method")
			w.Header().Set("Access-Control-Max-Age", "86400")

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Logging middleware for net/http
func LoggingMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)
			next.ServeHTTP(w, r)
		})
	}
}

// Recovery middleware for net/http
func RecoveryMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Printf("Panic recovered: %v", err)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// AuthMiddleware example for net/http
func AuthMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check for User-Id header
			userID := r.Header.Get("User-Id")
			if userID == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Set user ID in request context using a custom type
			type userIDKey string
			const userIDContextKey userIDKey = "userId"
			ctx := context.WithValue(r.Context(), userIDContextKey, userID)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}
