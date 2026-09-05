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

func TestPersistence_BeforeExecuteRunsBeforeHandler(t *testing.T) {
	exec := NewExecutor(
		WithRegistry(operation.NewRegistry()),
		WithNopTelemetry(),
	)

	var order []string
	persister := &FuncPersister[TestReq, TestResp]{
		BeforeExecuteFn: func(ctx *request.Context, req *TestReq) error {
			order = append(order, "before")
			return nil
		},
		AfterCommitFn: func(ctx *request.Context, req *TestReq, resp *TestResp, err error) error {
			order = append(order, "after")
			return nil
		},
	}

	ep := New[TestReq, TestResp](
		func(ctx *request.Context, req TestReq) (TestResp, error) {
			order = append(order, "handler")
			return TestResp{Status: "ok"}, nil
		},
		WithOperation(operation.Operation{ID: "test-persist-order", Method: "POST", Pattern: "/persist-order"}),
		WithBindingPlan(binding.Plan{Mode: binding.ModeJSON}),
	).WithPersister(persister)

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

	if len(order) != 3 || order[0] != "before" || order[1] != "handler" || order[2] != "after" {
		t.Fatalf("expected [before, handler, after], got %v", order)
	}
}

func TestPersistence_BeforeExecuteErrorAborts(t *testing.T) {
	exec := NewExecutor(
		WithRegistry(operation.NewRegistry()),
		WithNopTelemetry(),
	)

	mapper := response.NewMapperRegistry()
	mapper.Register(
		func(err error) bool { return true },
		func(err error) *response.Problem {
			return response.NewProblemWithCode(500, "Persist Failed", "PERSIST_FAILED")
		},
	)
	exec.ProblemMapper = mapper

	handlerCalled := false
	afterCalled := false
	persistErr := errors.New("insert failed")
	persister := &FuncPersister[TestReq, TestResp]{
		BeforeExecuteFn: func(ctx *request.Context, req *TestReq) error {
			return persistErr
		},
		AfterCommitFn: func(ctx *request.Context, req *TestReq, resp *TestResp, err error) error {
			afterCalled = true
			return nil
		},
	}

	ep := New[TestReq, TestResp](
		func(ctx *request.Context, req TestReq) (TestResp, error) {
			handlerCalled = true
			return TestResp{Status: "ok"}, nil
		},
		WithOperation(operation.Operation{ID: "test-persist-err", Method: "POST", Pattern: "/persist-err"}),
		WithBindingPlan(binding.Plan{Mode: binding.ModeJSON}),
	).WithPersister(persister)

	ctx := request.NewContext(context.Background(),
		request.WithBodySource(request.NewStringBodySource(`{"name":"test"}`)),
		request.WithMethod("POST"),
	)
	ft := newFT()
	transport := &FakeTransportAdapter{FT: ft}

	_, err := Execute(exec, ctx, ep, transport)
	if err == nil {
		t.Fatal("expected error from BeforeExecute failure")
	}
	if handlerCalled {
		t.Fatal("handler should not be called when BeforeExecute fails")
	}
	if afterCalled {
		t.Fatal("AfterCommit should not be called when BeforeExecute fails")
	}
}

func TestPersistence_AfterCommitRunsOnSuccess(t *testing.T) {
	exec := NewExecutor(
		WithRegistry(operation.NewRegistry()),
		WithNopTelemetry(),
	)

	var afterResp *TestResp
	var afterErr error
	persister := &FuncPersister[TestReq, TestResp]{
		AfterCommitFn: func(ctx *request.Context, req *TestReq, resp *TestResp, err error) error {
			afterResp = resp
			afterErr = err
			return nil
		},
	}

	ep := New[TestReq, TestResp](
		func(ctx *request.Context, req TestReq) (TestResp, error) {
			return TestResp{Status: "ok"}, nil
		},
		WithOperation(operation.Operation{ID: "test-after-success", Method: "POST", Pattern: "/after-success"}),
		WithBindingPlan(binding.Plan{Mode: binding.ModeJSON}),
	).WithPersister(persister)

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

	if afterResp == nil || afterResp.Status != "ok" {
		t.Fatalf("AfterCommit should receive the response, got %v", afterResp)
	}
	if afterErr != nil {
		t.Fatalf("AfterCommit err should be nil on success, got %v", afterErr)
	}
}

func TestPersistence_AfterCommitErrorIsBestEffort(t *testing.T) {
	exec := NewExecutor(
		WithRegistry(operation.NewRegistry()),
		WithNopTelemetry(),
	)

	afterErr := errors.New("update failed")
	persister := &FuncPersister[TestReq, TestResp]{
		AfterCommitFn: func(ctx *request.Context, req *TestReq, resp *TestResp, err error) error {
			return afterErr
		},
	}

	ep := New[TestReq, TestResp](
		func(ctx *request.Context, req TestReq) (TestResp, error) {
			return TestResp{Status: "ok"}, nil
		},
		WithOperation(operation.Operation{ID: "test-after-err", Method: "POST", Pattern: "/after-err"}),
		WithBindingPlan(binding.Plan{Mode: binding.ModeJSON}),
	).WithPersister(persister)

	ctx := request.NewContext(context.Background(),
		request.WithBodySource(request.NewStringBodySource(`{"name":"test"}`)),
		request.WithMethod("POST"),
	)
	ft := newFT()
	transport := &FakeTransportAdapter{FT: ft}

	// AfterCommit error should NOT propagate — it's best-effort.
	_, err := Execute(exec, ctx, ep, transport)
	if err != nil {
		t.Fatalf("AfterCommit error should not propagate: %v", err)
	}
}

func TestPersistence_NilPersisterIsNoOp(t *testing.T) {
	exec := NewExecutor(
		WithRegistry(operation.NewRegistry()),
		WithNopTelemetry(),
	)

	ep := New[TestReq, TestResp](
		func(ctx *request.Context, req TestReq) (TestResp, error) {
			return TestResp{Status: "ok"}, nil
		},
		WithOperation(operation.Operation{ID: "test-nil-persist", Method: "POST", Pattern: "/nil-persist"}),
		WithBindingPlan(binding.Plan{Mode: binding.ModeJSON}),
	) // No persister — should work fine

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
}
