package handlers

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	v2libGin "github.com/hmmftg/requestCore/v2/libGin"
	"github.com/hmmftg/requestCore/v2/renderers"
	v2routing "github.com/hmmftg/requestCore/v2/routing"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// BenchmarkEndpointDirectSuccess measures the overhead of a successful
// endpoint dispatch through the full lifecycle (parse, log, handle, render,
// finalize) using the Gin adapter.
func BenchmarkEndpointDirectSuccess(b *testing.B) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	err := GetEndpoint[struct{}, TestResp](
		router, nil, respHandler, "/bench",
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
			return TestResp{Status: "ok"}, nil
		},
	)
	if err != nil {
		b.Fatalf("GetEndpoint: %v", err)
	}

	req := httptest.NewRequest("GET", "/bench", nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
	}
}

// BenchmarkEndpointDirectProblem measures the overhead of an endpoint
// dispatch that returns an error (problem response).
func BenchmarkEndpointDirectProblem(b *testing.B) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	err := GetEndpoint[struct{}, TestResp](
		router, nil, respHandler, "/bench-fail",
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
			return TestResp{}, errors.New("benchmark error")
		},
	)
	if err != nil {
		b.Fatalf("GetEndpoint: %v", err)
	}

	req := httptest.NewRequest("GET", "/bench-fail", nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
	}
}

// BenchmarkEndpointFiveMiddleware measures the overhead of a successful
// endpoint dispatch through a chain of five middleware wrappers.
func BenchmarkEndpointFiveMiddleware(b *testing.B) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	noopMW := func(next v2routing.Handler) v2routing.Handler {
		return func(ctx *v2wf.RequestContext) error {
			return next(ctx)
		}
	}

	group := router.With(noopMW, noopMW, noopMW, noopMW, noopMW)
	err := GetEndpoint[struct{}, TestResp](
		group, nil, respHandler, "/bench-mw",
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
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
		engine.ServeHTTP(w, req)
	}
}

// BenchmarkBindJSON measures JSON body binding overhead for a POST endpoint.
func BenchmarkBindJSON(b *testing.B) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	err := PostEndpoint[CreateReq, CreateResp](
		router, nil, respHandler, "/bench-bind",
		func(req *CreateReq, trx *HandlerRequest[CreateReq, CreateResp]) (CreateResp, error) {
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
		engine.ServeHTTP(w, req)
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

// BenchmarkEnterpriseAddLogSuccess measures the overhead of the mandatory
// AddLog success path including the <title>-req entry and LogValuer
// projection check.
func BenchmarkEnterpriseAddLogSuccess(b *testing.B) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	err := GetEndpoint[struct{}, TestResp](
		router, nil, respHandler, "/bench-addlog",
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
			return TestResp{Status: "ok"}, nil
		},
	)
	if err != nil {
		b.Fatalf("GetEndpoint: %v", err)
	}

	req := httptest.NewRequest("GET", "/bench-addlog", nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
	}
}
