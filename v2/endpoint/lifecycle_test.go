package endpoint

import (
	"context"
	"errors"
	"testing"

	"github.com/hmmftg/requestCore/v2/binding"
	"github.com/hmmftg/requestCore/v2/operation"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/response"
)

func TestInitializer_RunsBeforeHandler(t *testing.T) {
	exec := NewExecutor(
		WithRegistry(operation.NewRegistry()),
		WithNopTelemetry(),
	)

	var order []string
	ep := New[TestReq, TestResp](
		func(ctx *request.Context, req TestReq) (TestResp, error) {
			order = append(order, "handler")
			return TestResp{Status: "ok"}, nil
		},
		WithOperation(operation.Operation{ID: "test-init-order", Method: "POST", Pattern: "/init-order"}),
		WithBindingPlan(binding.Plan{Mode: binding.ModeJSON}),
	).WithInitializer(func(ctx *request.Context, req *TestReq) error {
		order = append(order, "initializer")
		return nil
	})

	ctx := request.NewContext(context.Background(),
		request.WithBodySource(request.NewStringBodySource(`{"name":"test"}`)),
		request.WithMethod("POST"),
	)
	ft := newFT()
	transport := &FakeTransportAdapter{FT: ft}

	_, err := Execute(exec, ctx, ep, transport)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(order) != 2 || order[0] != "initializer" || order[1] != "handler" {
		t.Fatalf("expected [initializer, handler], got %v", order)
	}
}

func TestInitializer_ErrorAbortsRequest(t *testing.T) {
	exec := NewExecutor(
		WithRegistry(operation.NewRegistry()),
		WithNopTelemetry(),
	)

	mapper := response.NewMapperRegistry()
	mapper.Register(
		func(err error) bool { return true },
		func(err error) *response.Problem {
			return response.NewProblemWithCode(400, "Init Failed", "INIT_FAILED")
		},
	)
	exec.ProblemMapper = mapper

	handlerCalled := false
	initErr := errors.New("init failed")
	ep := New[TestReq, TestResp](
		func(ctx *request.Context, req TestReq) (TestResp, error) {
			handlerCalled = true
			return TestResp{Status: "ok"}, nil
		},
		WithOperation(operation.Operation{ID: "test-init-err", Method: "POST", Pattern: "/init-err"}),
		WithBindingPlan(binding.Plan{Mode: binding.ModeJSON}),
	).WithInitializer(func(ctx *request.Context, req *TestReq) error {
		return initErr
	})

	ctx := request.NewContext(context.Background(),
		request.WithBodySource(request.NewStringBodySource(`{"name":"test"}`)),
		request.WithMethod("POST"),
	)
	ft := newFT()
	transport := &FakeTransportAdapter{FT: ft}

	_, err := Execute(exec, ctx, ep, transport)
	if err == nil {
		t.Fatal("expected error from initializer failure")
	}
	if handlerCalled {
		t.Fatal("handler should not be called when initializer fails")
	}
}

func TestFinalizer_RunsOnSuccess(t *testing.T) {
	exec := NewExecutor(
		WithRegistry(operation.NewRegistry()),
		WithNopTelemetry(),
	)

	var finalizerCalled bool
	var finalizerErr error
	var finalizerResp *TestResp
	ep := New[TestReq, TestResp](
		func(ctx *request.Context, req TestReq) (TestResp, error) {
			return TestResp{Status: "ok"}, nil
		},
		WithOperation(operation.Operation{ID: "test-fin-success", Method: "POST", Pattern: "/fin-success"}),
		WithBindingPlan(binding.Plan{Mode: binding.ModeJSON}),
	).WithFinalizer(func(ctx *request.Context, req *TestReq, resp *TestResp, err error) {
		finalizerCalled = true
		finalizerErr = err
		finalizerResp = resp
	})

	ctx := request.NewContext(context.Background(),
		request.WithBodySource(request.NewStringBodySource(`{"name":"test"}`)),
		request.WithMethod("POST"),
	)
	ft := newFT()
	transport := &FakeTransportAdapter{FT: ft}

	_, err := Execute(exec, ctx, ep, transport)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !finalizerCalled {
		t.Fatal("finalizer should be called on success")
	}
	if finalizerErr != nil {
		t.Fatalf("finalizer err should be nil on success, got %v", finalizerErr)
	}
	if finalizerResp == nil || finalizerResp.Status != "ok" {
		t.Fatalf("finalizer should receive the response, got %v", finalizerResp)
	}
}

func TestFinalizer_RunsOnError(t *testing.T) {
	exec := NewExecutor(
		WithRegistry(operation.NewRegistry()),
		WithNopTelemetry(),
	)

	var finalizerCalled bool
	var finalizerErr error
	handlerErr := errors.New("handler failed")
	ep := New[TestReq, TestResp](
		func(ctx *request.Context, req TestReq) (TestResp, error) {
			return TestResp{}, handlerErr
		},
		WithOperation(operation.Operation{ID: "test-fin-error", Method: "POST", Pattern: "/fin-error"}),
		WithBindingPlan(binding.Plan{Mode: binding.ModeJSON}),
	).WithFinalizer(func(ctx *request.Context, req *TestReq, resp *TestResp, err error) {
		finalizerCalled = true
		finalizerErr = err
	})

	ctx := request.NewContext(context.Background(),
		request.WithBodySource(request.NewStringBodySource(`{"name":"test"}`)),
		request.WithMethod("POST"),
	)
	ft := newFT()
	transport := &FakeTransportAdapter{FT: ft}

	_, err := Execute(exec, ctx, ep, transport)
	if err == nil {
		t.Fatal("expected error from handler")
	}

	if !finalizerCalled {
		t.Fatal("finalizer should be called on error")
	}
	if finalizerErr == nil {
		t.Fatal("finalizer should receive the error")
	}
}

func TestFinalizer_PanicDoesNotPropagate(t *testing.T) {
	exec := NewExecutor(
		WithRegistry(operation.NewRegistry()),
		WithNopTelemetry(),
	)

	ep := New[TestReq, TestResp](
		func(ctx *request.Context, req TestReq) (TestResp, error) {
			return TestResp{Status: "ok"}, nil
		},
		WithOperation(operation.Operation{ID: "test-fin-panic", Method: "POST", Pattern: "/fin-panic"}),
		WithBindingPlan(binding.Plan{Mode: binding.ModeJSON}),
	).WithFinalizer(func(ctx *request.Context, req *TestReq, resp *TestResp, err error) {
		panic("finalizer panic")
	})

	ctx := request.NewContext(context.Background(),
		request.WithBodySource(request.NewStringBodySource(`{"name":"test"}`)),
		request.WithMethod("POST"),
	)
	ft := newFT()
	transport := &FakeTransportAdapter{FT: ft}

	// Should not panic — the executor recovers finalizer panics.
	_, err := Execute(exec, ctx, ep, transport)
	if err != nil {
		t.Fatalf("Execute should not return error from finalizer panic: %v", err)
	}
}

// TestReq is a test request type.
type TestReq struct {
	Name string `json:"name"`
}
