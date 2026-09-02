package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hmmftg/requestCore/v2/endpoint"
	"github.com/hmmftg/requestCore/v2/operation"
	"github.com/hmmftg/requestCore/v2/renderers"
	"github.com/hmmftg/requestCore/v2/request"
	v2routing "github.com/hmmftg/requestCore/v2/routing"
)

// benchExecutor creates an executor with nop telemetry for benchmarks.
func benchExecutor() *endpoint.Executor {
	return endpoint.NewExecutor(
		endpoint.WithRegistry(operation.NewRegistry()),
		endpoint.WithNopTelemetry(),
	)
}

// BenchmarkEndpointDirectSuccess measures the overhead of a successful
// endpoint dispatch through the full lifecycle (bind, execute, encode,
// commit) using the chi adapter.
func BenchmarkEndpointDirectSuccess(b *testing.B) {
	exec := benchExecutor()
	router := testRouter()

	ep := Get[struct{}, TestResp]("bench-success", "/bench",
		func(ctx *request.Context, req struct{}) (TestResp, error) {
			return TestResp{Status: "ok"}, nil
		},
	)
	if err := RegisterEndpoint(router, exec, ep); err != nil {
		b.Fatalf("RegisterEndpoint: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	req := httptest.NewRequest("GET", "/bench", nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		// Serve through the chi mux directly.
		router.Native().(http.Handler).ServeHTTP(w, req)
	}
}

// BenchmarkEndpointDirectProblem measures the overhead of an endpoint
// dispatch that returns an error (problem response).
func BenchmarkEndpointDirectProblem(b *testing.B) {
	exec := benchExecutor()
	router := testRouter()

	ep := Get[struct{}, TestResp]("bench-fail", "/bench-fail",
		func(ctx *request.Context, req struct{}) (TestResp, error) {
			return TestResp{}, errors.New("benchmark error")
		},
	)
	if err := RegisterEndpoint(router, exec, ep); err != nil {
		b.Fatalf("RegisterEndpoint: %v", err)
	}

	req := httptest.NewRequest("GET", "/bench-fail", nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.Native().(http.Handler).ServeHTTP(w, req)
	}
}

// BenchmarkEndpointFiveMiddleware measures the overhead of a successful
// endpoint dispatch through a chain of five middleware wrappers.
func BenchmarkEndpointFiveMiddleware(b *testing.B) {
	exec := benchExecutor()
	router := testRouter()

	noopMW := func(next v2routing.Handler) v2routing.Handler {
		return func(ctx *request.Context, transport v2routing.Transport) error {
			return next(ctx, transport)
		}
	}

	group := router.With(noopMW, noopMW, noopMW, noopMW, noopMW)
	err := GetEndpoint[struct{}, TestResp](group, exec, "/bench-mw",
		func(ctx *request.Context, req struct{}) (TestResp, error) {
			return TestResp{Status: "ok"}, nil
		},
	)
	if err != nil {
		b.Fatalf("GetEndpoint: %v", err)
	}

	req := httptest.NewRequest("GET", "/bench-mw", nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.Native().(http.Handler).ServeHTTP(w, req)
	}
}

// BenchmarkBindJSON measures JSON body binding overhead for a POST endpoint.
func BenchmarkBindJSON(b *testing.B) {
	exec := benchExecutor()
	router := testRouter()

	err := PostEndpoint[CreateReq, CreateResp](router, exec, "/bench-bind",
		func(ctx *request.Context, req CreateReq) (CreateResp, error) {
			return CreateResp{ID: "1", Name: req.Name}, nil
		},
	)
	if err != nil {
		b.Fatalf("PostEndpoint: %v", err)
	}

	body := `{"name":"alice"}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/bench-bind", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.Native().(http.Handler).ServeHTTP(w, req)
	}
}

// BenchmarkRenderJSON measures the JSON renderer encode overhead.
func BenchmarkRenderJSON(b *testing.B) {
	r := renderers.JSONRenderer{}
	data := TestResp{Status: "ok"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.Encode(data)
	}
}

// BenchmarkEndpointTelemetrySuccess measures the overhead of a
// successful endpoint dispatch with telemetry enabled (slog sink).
func BenchmarkEndpointTelemetrySuccess(b *testing.B) {
	exec := endpoint.NewExecutor(
		endpoint.WithRegistry(operation.NewRegistry()),
		// Default executor uses SlogSink(nil) which is observable.
	)
	router := testRouter()

	ep := Get[struct{}, TestResp]("bench-telemetry", "/bench-telemetry",
		func(ctx *request.Context, req struct{}) (TestResp, error) {
			return TestResp{Status: "ok"}, nil
		},
	)
	if err := RegisterEndpoint(router, exec, ep); err != nil {
		b.Fatalf("RegisterEndpoint: %v", err)
	}

	req := httptest.NewRequest("GET", "/bench-telemetry", nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.Native().(http.Handler).ServeHTTP(w, req)
	}
}
