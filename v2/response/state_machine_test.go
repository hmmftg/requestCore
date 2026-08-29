package response

import (
	"errors"
	"sync"
	"testing"
)

func TestCommitMachine_FullCycle(t *testing.T) {
	m := NewCommitMachine()
	if m.State() != StateOpen {
		t.Fatalf("expected Open, got %s", m.State())
	}

	if err := m.Prepare(200); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if m.State() != StatePrepared {
		t.Fatalf("expected Prepared, got %s", m.State())
	}
	if m.Status() != 200 {
		t.Fatalf("expected status 200, got %d", m.Status())
	}

	if err := m.RunHooks(nil); err != nil {
		t.Fatalf("RunHooks: %v", err)
	}
	if m.State() != StateHooksRun {
		t.Fatalf("expected HooksRun, got %s", m.State())
	}

	if err := m.MarkDurable(); err != nil {
		t.Fatalf("MarkDurable: %v", err)
	}
	if m.State() != StateDurable {
		t.Fatalf("expected Durable, got %s", m.State())
	}

	if err := m.Commit(200); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if m.State() != StateCommitted {
		t.Fatalf("expected Committed, got %s", m.State())
	}
	if !m.Committed() {
		t.Fatal("expected Committed()=true")
	}

	if err := m.Observe(); err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if m.State() != StateObserved {
		t.Fatalf("expected Observed, got %s", m.State())
	}
	if !m.Committed() {
		t.Fatal("expected Committed()=true in Observed")
	}
}

func TestCommitMachine_PrepareRequiresOpen(t *testing.T) {
	m := NewCommitMachine()
	_ = m.Prepare(200)
	err := m.Prepare(201)
	if err == nil {
		t.Fatal("expected error on second Prepare")
	}
}

func TestCommitMachine_RunHooksIdempotent(t *testing.T) {
	m := NewCommitMachine()
	_ = m.Prepare(200)
	called := 0
	hooks := []func() error{
		func() error { called++; return nil },
	}
	if err := m.RunHooks(hooks); err != nil {
		t.Fatalf("first RunHooks: %v", err)
	}
	if called != 1 {
		t.Fatalf("expected 1 call, got %d", called)
	}
	// Second call should be a no-op.
	if err := m.RunHooks(hooks); err != nil {
		t.Fatalf("second RunHooks: %v", err)
	}
	if called != 1 {
		t.Fatalf("expected still 1 call, got %d", called)
	}
}

func TestCommitMachine_HookErrorTransitionsToFailed(t *testing.T) {
	m := NewCommitMachine()
	_ = m.Prepare(200)
	hookErr := errors.New("hook failed")
	hooks := []func() error{
		func() error { return hookErr },
	}
	err := m.RunHooks(hooks)
	if !errors.Is(err, hookErr) {
		t.Fatalf("expected hookErr, got %v", err)
	}
	if m.State() != StateFailed {
		t.Fatalf("expected Failed, got %s", m.State())
	}
	if !m.Failed() {
		t.Fatal("expected Failed()=true")
	}
	if !errors.Is(m.FailError(), hookErr) {
		t.Fatalf("expected failErr=hookErr, got %v", m.FailError())
	}
}

func TestCommitMachine_AllHooksRunOnFirstError(t *testing.T) {
	m := NewCommitMachine()
	_ = m.Prepare(200)
	order := []string{}
	hooks := []func() error{
		func() error { order = append(order, "first"); return errors.New("boom") },
		func() error { order = append(order, "second"); return nil },
		func() error { order = append(order, "third"); return nil },
	}
	_ = m.RunHooks(hooks)
	if len(order) != 3 {
		t.Fatalf("expected all 3 hooks to run, got %d", len(order))
	}
	if order[0] != "first" || order[1] != "second" || order[2] != "third" {
		t.Fatalf("expected [first, second, third], got %v", order)
	}
}

func TestCommitMachine_NilHooksSkipped(t *testing.T) {
	m := NewCommitMachine()
	_ = m.Prepare(200)
	called := 0
	hooks := []func() error{
		nil,
		func() error { called++; return nil },
		nil,
	}
	if err := m.RunHooks(hooks); err != nil {
		t.Fatalf("RunHooks: %v", err)
	}
	if called != 1 {
		t.Fatalf("expected 1 call (nil hooks skipped), got %d", called)
	}
}

func TestCommitMachine_FirstCommitWins(t *testing.T) {
	m := NewCommitMachine()
	_ = m.Prepare(200)
	_ = m.RunHooks(nil)
	_ = m.MarkDurable()
	if err := m.Commit(200); err != nil {
		t.Fatalf("first Commit: %v", err)
	}
	if m.Status() != 200 {
		t.Fatalf("expected status 200, got %d", m.Status())
	}
	// Second commit with different status should be a no-op.
	if err := m.Commit(500); err != nil {
		t.Fatalf("second Commit: %v", err)
	}
	if m.Status() != 200 {
		t.Fatalf("expected status still 200, got %d", m.Status())
	}
}

func TestCommitMachine_CommitRequiresDurable(t *testing.T) {
	m := NewCommitMachine()
	_ = m.Prepare(200)
	err := m.Commit(200)
	if err == nil {
		t.Fatal("expected error on Commit without MarkDurable")
	}
}

func TestCommitMachine_MarkDurableRequiresHooksRun(t *testing.T) {
	m := NewCommitMachine()
	_ = m.Prepare(200)
	err := m.MarkDurable()
	if err == nil {
		t.Fatal("expected error on MarkDurable without RunHooks")
	}
}

func TestCommitMachine_ObserveRequiresCommitted(t *testing.T) {
	m := NewCommitMachine()
	err := m.Observe()
	if err == nil {
		t.Fatal("expected error on Observe without Commit")
	}
}

func TestCommitMachine_FailFromOpen(t *testing.T) {
	m := NewCommitMachine()
	failErr := errors.New("open failure")
	if err := m.Fail(failErr); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if m.State() != StateFailed {
		t.Fatalf("expected Failed, got %s", m.State())
	}
	if !errors.Is(m.FailError(), failErr) {
		t.Fatalf("expected failErr, got %v", m.FailError())
	}
}

func TestCommitMachine_FailFromPrepared(t *testing.T) {
	m := NewCommitMachine()
	_ = m.Prepare(200)
	if err := m.Fail(errors.New("prep failure")); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if m.State() != StateFailed {
		t.Fatalf("expected Failed, got %s", m.State())
	}
}

func TestCommitMachine_FailFromDurable(t *testing.T) {
	m := NewCommitMachine()
	_ = m.Prepare(200)
	_ = m.RunHooks(nil)
	_ = m.MarkDurable()
	if err := m.Fail(errors.New("durable failure")); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if m.State() != StateFailed {
		t.Fatalf("expected Failed, got %s", m.State())
	}
}

func TestCommitMachine_FailAfterCommitIsNoOp(t *testing.T) {
	m := NewCommitMachine()
	_ = m.Prepare(200)
	_ = m.RunHooks(nil)
	_ = m.MarkDurable()
	_ = m.Commit(200)
	originalState := m.State()
	if err := m.Fail(errors.New("post-commit failure")); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	// State should not change.
	if m.State() != originalState {
		t.Fatalf("expected state unchanged, got %s", m.State())
	}
	if m.FailError() != nil {
		t.Fatalf("expected nil failErr after post-commit Fail, got %v", m.FailError())
	}
}

func TestCommitMachine_FailAfterObserveIsNoOp(t *testing.T) {
	m := NewCommitMachine()
	_ = m.Prepare(200)
	_ = m.RunHooks(nil)
	_ = m.MarkDurable()
	_ = m.Commit(200)
	_ = m.Observe()
	if err := m.Fail(errors.New("post-observe failure")); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if m.State() != StateObserved {
		t.Fatalf("expected Observed, got %s", m.State())
	}
}

func TestCommitMachine_FailWhenAlreadyFailedKeepsFirstError(t *testing.T) {
	m := NewCommitMachine()
	firstErr := errors.New("first failure")
	_ = m.Fail(firstErr)
	_ = m.Fail(errors.New("second failure"))
	if !errors.Is(m.FailError(), firstErr) {
		t.Fatalf("expected first error retained, got %v", m.FailError())
	}
}

func TestCommitMachine_ConcurrentCommit(t *testing.T) {
	m := NewCommitMachine()
	_ = m.Prepare(200)
	_ = m.RunHooks(nil)
	_ = m.MarkDurable()

	var wg sync.WaitGroup
	statuses := make(chan int, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(status int) {
			defer wg.Done()
			_ = m.Commit(status)
			statuses <- status
		}(200 + i)
	}
	wg.Wait()
	close(statuses)

	// Exactly one commit should have won; status should be one of the
	// submitted values and should not change.
	finalStatus := m.Status()
	if finalStatus < 200 || finalStatus >= 210 {
		t.Fatalf("expected status in [200, 209], got %d", finalStatus)
	}
	// Calling Commit again should not change the status.
	_ = m.Commit(999)
	if m.Status() != finalStatus {
		t.Fatalf("expected status to remain %d, got %d", finalStatus, m.Status())
	}
}

func TestCommitMachine_PanicInHook(t *testing.T) {
	m := NewCommitMachine()
	_ = m.Prepare(200)
	hooks := []func() error{
		func() error { panic("hook panic") },
	}
	// The state machine itself does not recover panics; the caller
	// (response handler) is responsible for recovery. We verify that
	// an unrecovered panic propagates.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic to propagate")
		}
		// After panic, state is indeterminate but should not be Committed.
		if m.Committed() {
			t.Fatal("expected not committed after panic")
		}
	}()
	_ = m.RunHooks(hooks)
}

func TestCommitMachine_StateString(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateOpen, "Open"},
		{StatePrepared, "Prepared"},
		{StateHooksRun, "HooksRun"},
		{StateDurable, "Durable"},
		{StateCommitted, "Committed"},
		{StateObserved, "Observed"},
		{StateFailed, "Failed"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}
