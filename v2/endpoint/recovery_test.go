package endpoint

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/hmmftg/requestCore/v2/binding"
	"github.com/hmmftg/requestCore/v2/operation"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/response"
)

func TestRecovery_DefaultPanicProduces500(t *testing.T) {
	exec := NewExecutor(
		WithRegistry(operation.NewRegistry()),
		WithNopTelemetry(),
	)

	ep := New[TestReq, TestResp](
		func(ctx *request.Context, req TestReq) (TestResp, error) {
			panic("boom")
		},
		WithOperation(operation.Operation{ID: "test-panic-default", Method: "POST", Pattern: "/panic-default"}),
		WithBindingPlan(binding.Plan{Mode: binding.ModeJSON}),
	)

	ctx := request.NewContext(context.Background(),
		request.WithBodySource(request.NewStringBodySource(`{"name":"test"}`)),
		request.WithMethod("POST"),
	)
	ft := newFT()
	transport := &FakeTransportAdapter{FT: ft}

	_, err := Execute(exec, ctx, ep, transport)
	if err == nil {
		t.Fatal("expected error from panic")
	}

	rec := ft.Recorder()
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestRecovery_CustomHandlerMapsPanicToError(t *testing.T) {
	mapper := response.NewMapperRegistry()
	mapper.Register(
		func(err error) bool { return true },
		func(err error) *response.Problem {
			return response.NewProblemWithCode(http.StatusConflict, "Conflict", "CONFLICT")
		},
	)
	exec := NewExecutor(
		WithRegistry(operation.NewRegistry()),
		WithNopTelemetry(),
		WithProblemMapper(mapper),
	)

	domainErr := errors.New("domain-specific error from panic")
	ep := New[TestReq, TestResp](
		func(ctx *request.Context, req TestReq) (TestResp, error) {
			panic("custom panic")
		},
		WithOperation(operation.Operation{ID: "test-panic-custom", Method: "POST", Pattern: "/panic-custom"}),
		WithBindingPlan(binding.Plan{Mode: binding.ModeJSON}),
		WithRecoveryHandler(func(panicVal any) error {
			// Verify we receive the panic value
			if panicVal != "custom panic" {
				t.Fatalf("expected panic value 'custom panic', got %v", panicVal)
			}
			return domainErr
		}),
	)

	ctx := request.NewContext(context.Background(),
		request.WithBodySource(request.NewStringBodySource(`{"name":"test"}`)),
		request.WithMethod("POST"),
	)
	ft := newFT()
	transport := &FakeTransportAdapter{FT: ft}

	_, err := Execute(exec, ctx, ep, transport)
	if err == nil {
		t.Fatal("expected error from panic")
	}

	rec := ft.Recorder()
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 (mapped), got %d", rec.Code)
	}
}

func TestRecovery_CustomHandlerReturningNilUsesGenericError(t *testing.T) {
	exec := NewExecutor(
		WithRegistry(operation.NewRegistry()),
		WithNopTelemetry(),
	)

	ep := New[TestReq, TestResp](
		func(ctx *request.Context, req TestReq) (TestResp, error) {
			panic("nil-return panic")
		},
		WithOperation(operation.Operation{ID: "test-panic-nil", Method: "POST", Pattern: "/panic-nil"}),
		WithBindingPlan(binding.Plan{Mode: binding.ModeJSON}),
		WithRecoveryHandler(func(panicVal any) error {
			return nil // return nil → generic panic error
		}),
	)

	ctx := request.NewContext(context.Background(),
		request.WithBodySource(request.NewStringBodySource(`{"name":"test"}`)),
		request.WithMethod("POST"),
	)
	ft := newFT()
	transport := &FakeTransportAdapter{FT: ft}

	_, err := Execute(exec, ctx, ep, transport)
	if err == nil {
		t.Fatal("expected error from panic")
	}

	rec := ft.Recorder()
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 (generic), got %d", rec.Code)
	}
}
