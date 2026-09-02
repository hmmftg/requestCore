package routing_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	chi "github.com/go-chi/chi/v5"
	"github.com/gofiber/fiber/v2"

	v2libChi "github.com/hmmftg/requestCore/v2/libChi"
	v2libFiber "github.com/hmmftg/requestCore/v2/libFiber"
	v2libGin "github.com/hmmftg/requestCore/v2/libGin"
	v2libNetHttp "github.com/hmmftg/requestCore/v2/libNetHttp"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/routing"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// noopHandler is a minimal handler that writes a 200 response directly
// through the transport, used for adapter overhead benchmarks.
func noopHandler() routing.Handler {
	return func(ctx *request.Context, transport routing.Transport) error {
		return transport.WriteResponse(200, "text/plain", nil, []byte("ok"))
	}
}

// BenchmarkAdapterGin measures one HTTP round trip through the Gin adapter.
func BenchmarkAdapterGin(b *testing.B) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	if err := router.Get("/bench", noopHandler()); err != nil {
		b.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/bench", nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
	}
}

// BenchmarkAdapterFiber measures one HTTP round trip through the Fiber adapter.
func BenchmarkAdapterFiber(b *testing.B) {
	app := fiber.New()
	router := v2libFiber.NewRouter(app)
	if err := router.Get("/bench", noopHandler()); err != nil {
		b.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/bench", nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = app.Test(req)
	}
}

// BenchmarkAdapterChi measures one HTTP round trip through the chi adapter.
func BenchmarkAdapterChi(b *testing.B) {
	router := v2libChi.NewRouter()
	if err := router.Get("/bench", noopHandler()); err != nil {
		b.Fatal(err)
	}
	mux := router.Native().(*chi.Mux)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/bench", nil)
		mux.ServeHTTP(w, req)
	}
}

// BenchmarkAdapterNetHTTP measures one HTTP round trip through the net/http adapter.
func BenchmarkAdapterNetHTTP(b *testing.B) {
	router := v2libNetHttp.NewRouter()
	if err := router.Get("/bench", noopHandler()); err != nil {
		b.Fatal(err)
	}
	mux := router.Native().(http.Handler)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/bench", nil)
		mux.ServeHTTP(w, req)
	}
}
