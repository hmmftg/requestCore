package nextadapter

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hmmftg/requestCore/v2/binding"
	"github.com/hmmftg/requestCore/v2/internal/endpoint"
	v2libChi "github.com/hmmftg/requestCore/v2/libChi"
	v2libNetHttp "github.com/hmmftg/requestCore/v2/libNetHttp"
	"github.com/hmmftg/requestCore/v2/operation"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/routing"
	"github.com/hmmftg/requestCore/v2/validation"
)

// BenchmarkNextAdapter_ChiSuccess measures the cost of a full
// successful request through chi and the new executor.
func BenchmarkNextAdapter_ChiSuccess(b *testing.B) {
	router := v2libChi.NewRouter()
	exec := endpoint.NewExecutor(endpoint.WithRegistry(operation.NewRegistry()), endpoint.WithNopTelemetry())
	ep := endpoint.New[pingReq, pingResp](
		func(ctx *request.Context, req pingReq) (pingResp, error) {
			return pingResp{Message: "pong"}, nil
		},
		endpoint.WithOperation(operation.Operation{ID: "ping", Method: "GET", Pattern: "/ping"}),
	)
	if err := Register(router, exec, "GET", "/ping", ep); err != nil {
		b.Fatalf("Register: %v", err)
	}
	native := router.Native().(http.Handler)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/ping", nil)
		native.ServeHTTP(w, req)
	}
}

// BenchmarkNextAdapter_NetHTTPSuccess measures the cost of a full
// successful request through net/http ServeMux and the new executor.
func BenchmarkNextAdapter_NetHTTPSuccess(b *testing.B) {
	router := v2libNetHttp.NewRouter()
	exec := endpoint.NewExecutor(endpoint.WithRegistry(operation.NewRegistry()), endpoint.WithNopTelemetry())
	ep := endpoint.New[pingReq, pingResp](
		func(ctx *request.Context, req pingReq) (pingResp, error) {
			return pingResp{Message: "pong"}, nil
		},
		endpoint.WithOperation(operation.Operation{ID: "ping", Method: "GET", Pattern: "/ping"}),
	)
	if err := Register(router, exec, "GET", "/ping", ep); err != nil {
		b.Fatalf("Register: %v", err)
	}
	native := router.Native().(http.Handler)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/ping", nil)
		native.ServeHTTP(w, req)
	}
}

// BenchmarkNextAdapter_ChiJSONBindValidate measures the cost of JSON
// binding plus validation through chi and the new executor.
func BenchmarkNextAdapter_ChiJSONBindValidate(b *testing.B) {
	router := v2libChi.NewRouter()
	exec := endpoint.NewExecutor(endpoint.WithRegistry(operation.NewRegistry()), endpoint.WithNopTelemetry())
	ep := endpoint.New[createUserReq, createUserResp](
		func(ctx *request.Context, req createUserReq) (createUserResp, error) {
			return createUserResp{ID: "1", Name: req.Name, Email: req.Email}, nil
		},
		endpoint.WithOperation(operation.Operation{ID: "create", Method: "POST", Pattern: "/users"}),
		endpoint.WithBindingPlan(binding.DefaultJSONPlan),
		endpoint.WithValidator(validation.New()),
		endpoint.WithSuccessStatus(http.StatusCreated),
	)
	if err := Register(router, exec, "POST", "/users", ep); err != nil {
		b.Fatalf("Register: %v", err)
	}
	native := router.Native().(http.Handler)
	body := `{"name":"Alice","email":"alice@example.com","age":30}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/users", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		native.ServeHTTP(w, req)
	}
}

// Ensure routing import is used.
var _ = routing.ValidatePattern
