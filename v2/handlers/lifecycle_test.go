package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hmmftg/requestCore/v2/endpoint"
	v2libChi "github.com/hmmftg/requestCore/v2/libChi"
	"github.com/hmmftg/requestCore/v2/operation"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/routing"
)

type lifecycleReq struct {
	Name string `json:"name"`
}

type lifecycleResp struct {
	Status string `json:"status"`
}

func lifecycleExec() *endpoint.Executor {
	return endpoint.NewExecutor(
		endpoint.WithRegistry(operation.NewRegistry()),
		endpoint.WithNopTelemetry(),
	)
}

func lifecycleRouter() routing.Router {
	r := v2libChi.NewRouter()
	mapper := response.DefaultMapperRegistry()
	r.SetErrorHandler(func(ctx *request.Context, transport routing.Transport, err error) {
		_ = response.WriteProblemFromError(ctx, transport, mapper, err)
	})
	return r
}

func TestHandlers_InitializerAndFinalizer(t *testing.T) {
	exec := lifecycleExec()
	r := lifecycleRouter()

	var order []string
	ep := Post[lifecycleReq, lifecycleResp]("lifecycle-test", "/lifecycle",
		func(ctx *request.Context, req lifecycleReq) (lifecycleResp, error) {
			order = append(order, "handler")
			return lifecycleResp{Status: "ok"}, nil
		},
	).WithInitializer(func(ctx *request.Context, req *lifecycleReq) error {
		order = append(order, "initializer")
		return nil
	}).WithFinalizer(func(ctx *request.Context, req *lifecycleReq, resp *lifecycleResp, err error) {
		order = append(order, "finalizer")
	})

	if err := RegisterEndpoint(r, exec, ep); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	srv := httptest.NewServer(r.Native().(http.Handler))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/lifecycle", "application/json", strings.NewReader(`{"name":"test"}`))
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if len(order) != 3 || order[0] != "initializer" || order[1] != "handler" || order[2] != "finalizer" {
		t.Fatalf("expected [initializer, handler, finalizer], got %v", order)
	}
}

func TestHandlers_Persister(t *testing.T) {
	exec := lifecycleExec()
	r := lifecycleRouter()

	var beforeCalled, afterCalled bool
	persister := &endpoint.FuncPersister[lifecycleReq, lifecycleResp]{
		BeforeExecuteFn: func(ctx *request.Context, req *lifecycleReq) error {
			beforeCalled = true
			return nil
		},
		AfterCommitFn: func(ctx *request.Context, req *lifecycleReq, resp *lifecycleResp, err error) error {
			afterCalled = true
			return nil
		},
	}

	ep := Post[lifecycleReq, lifecycleResp]("persist-test", "/persist",
		func(ctx *request.Context, req lifecycleReq) (lifecycleResp, error) {
			return lifecycleResp{Status: "ok"}, nil
		},
	).WithPersister(persister)

	if err := RegisterEndpoint(r, exec, ep); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	srv := httptest.NewServer(r.Native().(http.Handler))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/persist", "application/json", strings.NewReader(`{"name":"test"}`))
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if !beforeCalled {
		t.Fatal("BeforeExecute should be called")
	}
	if !afterCalled {
		t.Fatal("AfterCommit should be called")
	}
}

func TestHandlers_Tracing(t *testing.T) {
	exec := lifecycleExec()
	r := lifecycleRouter()

	ep := Get[struct{}, lifecycleResp]("trace-test", "/trace",
		func(ctx *request.Context, req struct{}) (lifecycleResp, error) {
			return lifecycleResp{Status: "ok"}, nil
		},
	).WithTracing("test-span")

	if err := RegisterEndpoint(r, exec, ep); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	srv := httptest.NewServer(r.Native().(http.Handler))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/trace")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandlers_RecoveryHandler(t *testing.T) {
	exec := lifecycleExec()
	r := lifecycleRouter()

	ep := Get[struct{}, lifecycleResp]("recovery-test", "/recovery",
		func(ctx *request.Context, req struct{}) (lifecycleResp, error) {
			panic("test panic")
		},
	).WithRecoveryHandler(func(panicVal any) error {
		// Return a domain-specific error
		return errDomainPanic
	})

	// Register a problem mapper for the domain error
	exec.ProblemMapper.Register(
		func(err error) bool { return err == errDomainPanic },
		func(err error) *response.Problem {
			return response.NewProblemWithCode(http.StatusConflict, "Domain Panic", "DOMAIN_PANIC")
		},
	)

	if err := RegisterEndpoint(r, exec, ep); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	srv := httptest.NewServer(r.Native().(http.Handler))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/recovery")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 (mapped panic), got %d", resp.StatusCode)
	}
}

var errDomainPanic = &domainPanicError{}

type domainPanicError struct{}

func (e *domainPanicError) Error() string { return "domain panic" }
