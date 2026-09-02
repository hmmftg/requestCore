package endpoint

import (
	"net/http"
	"testing"

	"github.com/hmmftg/requestCore/v2/binding"
	"github.com/hmmftg/requestCore/v2/operation"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/request/faketransport"
	"github.com/hmmftg/requestCore/v2/validation"
)

// BenchmarkEndpointDirectSuccess measures the cost of executing an
// endpoint with no binding and no validation (handler + encode +
// commit lifecycle only).
func BenchmarkEndpointDirectSuccess(b *testing.B) {
	exec := NewExecutor(WithRegistry(operation.NewRegistry()), WithNopTelemetry())
	ep := New[PingReq, PingResp](
		func(ctx *request.Context, req PingReq) (PingResp, error) {
			return PingResp{Message: "pong"}, nil
		},
		WithOperation(operation.Operation{ID: "ping", Method: "GET", Pattern: "/ping"}),
	)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ft := faketransport.New("GET", "/ping")
		transport := &FakeTransportAdapter{FT: ft}
		_, _ = Execute(exec, ft.Context(), ep, transport)
	}
}

// BenchmarkEndpointDirectProblem measures the cost of executing an
// endpoint that returns a mapped problem (handler error path).
func BenchmarkEndpointDirectProblem(b *testing.B) {
	exec := NewExecutor(WithRegistry(operation.NewRegistry()), WithNopTelemetry())
	ep := New[problemReq, problemResp](
		func(ctx *request.Context, req problemReq) (problemResp, error) {
			return problemResp{}, errBenchmark
		},
		WithOperation(operation.Operation{ID: "prob", Method: "POST", Pattern: "/prob"}),
		WithBindingPlan(binding.DefaultJSONPlan),
		WithValidator(validation.New()),
	)
	body := `{"mode":"ok"}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ft := faketransport.New("POST", "/prob", faketransport.WithBody(body))
		transport := &FakeTransportAdapter{FT: ft}
		_, _ = Execute(exec, ft.Context(), ep, transport)
	}
}

// BenchmarkEndpointBindJSON measures the cost of JSON binding only
// (no validation, handler returns immediately).
func BenchmarkEndpointBindJSON(b *testing.B) {
	exec := NewExecutor(WithRegistry(operation.NewRegistry()), WithNopTelemetry())
	ep := New[CreateUserReq, CreateUserResp](
		func(ctx *request.Context, req CreateUserReq) (CreateUserResp, error) {
			return CreateUserResp{ID: "1", Name: req.Name, Email: req.Email}, nil
		},
		WithOperation(operation.Operation{ID: "create", Method: "POST", Pattern: "/users"}),
		WithBindingPlan(binding.DefaultJSONPlan),
		WithSuccessStatus(http.StatusCreated),
	)
	body := `{"name":"Alice","email":"alice@example.com","age":30}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ft := faketransport.New("POST", "/users", faketransport.WithBody(body))
		transport := &FakeTransportAdapter{FT: ft}
		_, _ = Execute(exec, ft.Context(), ep, transport)
	}
}

// BenchmarkEndpointBindAndValidate measures the cost of JSON binding
// plus validation.
func BenchmarkEndpointBindAndValidate(b *testing.B) {
	exec := NewExecutor(WithRegistry(operation.NewRegistry()), WithNopTelemetry())
	ep := New[CreateUserReq, CreateUserResp](
		func(ctx *request.Context, req CreateUserReq) (CreateUserResp, error) {
			return CreateUserResp{ID: "1", Name: req.Name, Email: req.Email}, nil
		},
		WithOperation(operation.Operation{ID: "create", Method: "POST", Pattern: "/users"}),
		WithBindingPlan(binding.DefaultJSONPlan),
		WithValidator(validation.New()),
		WithSuccessStatus(http.StatusCreated),
	)
	body := `{"name":"Alice","email":"alice@example.com","age":30}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ft := faketransport.New("POST", "/users", faketransport.WithBody(body))
		transport := &FakeTransportAdapter{FT: ft}
		_, _ = Execute(exec, ft.Context(), ep, transport)
	}
}

// BenchmarkEndpointRenderJSON measures the cost of encoding a response
// to JSON (handler returns a populated struct, encode + write).
func BenchmarkEndpointRenderJSON(b *testing.B) {
	exec := NewExecutor(WithRegistry(operation.NewRegistry()), WithNopTelemetry())
	ep := New[PingReq, PingResp](
		func(ctx *request.Context, req PingReq) (PingResp, error) {
			return PingResp{Message: "pong"}, nil
		},
		WithOperation(operation.Operation{ID: "ping", Method: "GET", Pattern: "/ping"}),
	)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ft := faketransport.New("GET", "/ping")
		transport := &FakeTransportAdapter{FT: ft}
		_, _ = Execute(exec, ft.Context(), ep, transport)
	}
}

// errBenchmark is a sentinel error for the problem benchmark.
var errBenchmark = &benchmarkError{}

type benchmarkError struct{}

func (e *benchmarkError) Error() string { return "benchmark error" }
