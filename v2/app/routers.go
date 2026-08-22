package app

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
	"github.com/go-chi/chi/v5"

	v2libChi "github.com/hmmftg/requestCore/v2/libChi"
	v2libFiber "github.com/hmmftg/requestCore/v2/libFiber"
	v2libGin "github.com/hmmftg/requestCore/v2/libGin"
	v2libNetHttp "github.com/hmmftg/requestCore/v2/libNetHttp"
	"github.com/hmmftg/requestCore/v2/routing"
)

// createGinRouter creates a Gin router and returns it as a routing.Router.
func createGinRouter() (routing.Router, error) {
	engine := gin.New()
	return v2libGin.NewRouter(engine), nil
}

// createFiberRouter creates a Fiber router and returns it as a routing.Router.
func createFiberRouter() (routing.Router, error) {
	app := fiber.New()
	return v2libFiber.NewRouter(app), nil
}

// createChiRouter creates a chi router and returns it as a routing.Router.
func createChiRouter() (routing.Router, error) {
	return v2libChi.NewRouter(), nil
}

// createNetHTTPRouter creates a net/http router and returns it as a routing.Router.
func createNetHTTPRouter() (routing.Router, error) {
	return v2libNetHttp.NewRouter(), nil
}

// startFiber starts a Fiber app on the given address.
func startFiber(native any, addr string) error {
	if app, ok := native.(*fiber.App); ok {
		return app.Listen(addr)
	}
	return nil
}

// shutdownFiber shuts down a Fiber app.
func shutdownFiber(native any, ctx context.Context) error {
	if app, ok := native.(*fiber.App); ok {
		return app.ShutdownWithContext(ctx)
	}
	return nil
}

// Ensure imports are used.
var _ chi.Router = (*chi.Mux)(nil)
var _ http.Handler = (*chi.Mux)(nil)
