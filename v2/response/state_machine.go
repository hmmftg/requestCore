package response

import (
	"errors"
	"sync"
)

// State represents a phase in the response commit state machine.
//
// The canonical lifecycle is:
//
//	Open → Prepared → HooksRun → Durable → Committed → Observed
//
// Failure transitions:
//
//	Open/Prepared/HooksRun/Durable → Failed
//	Committed/Observed             → cannot write again; record failure only
//
// Rules enforced by CommitMachine:
//  1. The first terminal commit wins atomically.
//  2. Strict hook failure discards the pending success and transitions
//     to Failed.
//  3. Best-effort hook failure records telemetry and allows progression.
//  4. Pre-transport interceptor failure prevents success transport
//     commit and transitions to Failed.
//  5. Transport failure transitions to Failed; no fallback body is
//     attempted on a partially written transport.
//  6. Post-commit failures cannot alter the response and are recorded only.
//  7. Already-committed or observed states reject further writes.
type State int

const (
	// StateOpen is the initial state. No response has been prepared.
	StateOpen State = iota

	// StatePrepared means the body has been encoded and canonical
	// headers/status are known, but hooks have not yet run.
	StatePrepared

	// StateHooksRun means before-commit hooks have executed exactly once.
	StateHooksRun

	// StateDurable means required pre-transport persistence and
	// interceptors have succeeded. The response is ready for transport.
	StateDurable

	// StateCommitted means the adapter transport write has been accepted.
	// No further response writes are allowed.
	StateCommitted

	// StateObserved means post-commit observers have completed (best-effort).
	// This is the terminal success state.
	StateObserved

	// StateFailed means an error occurred before commit. The response
	// was not sent as a success. This is the terminal failure state.
	StateFailed
)

// String returns a human-readable name for the state.
func (s State) String() string {
	switch s {
	case StateOpen:
		return "Open"
	case StatePrepared:
		return "Prepared"
	case StateHooksRun:
		return "HooksRun"
	case StateDurable:
		return "Durable"
	case StateCommitted:
		return "Committed"
	case StateObserved:
		return "Observed"
	case StateFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// CommitMachine implements the formal response commit state machine.
// It is safe for concurrent use.
//
// CommitMachine is additive to the existing response package and does
// not replace the existing webFramework.CommitState. It will be wired
// into the canonical response path in Tranche 4.
type CommitMachine struct {
	mu        sync.RWMutex
	state     State
	status    int
	hooksRan  bool
	failErr   error
}

// NewCommitMachine creates a CommitMachine in StateOpen.
func NewCommitMachine() *CommitMachine {
	return &CommitMachine{
		state: StateOpen,
	}
}

// State returns the current state.
func (m *CommitMachine) State() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// Status returns the committed status code, or 0 if not yet committed.
func (m *CommitMachine) Status() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// Committed reports whether the response has been committed (StateCommitted
// or StateObserved).
func (m *CommitMachine) Committed() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state == StateCommitted || m.state == StateObserved
}

// Failed reports whether the state machine is in StateFailed.
func (m *CommitMachine) Failed() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state == StateFailed
}

// FailError returns the error that caused the transition to Failed,
// or nil if not failed.
func (m *CommitMachine) FailError() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.failErr
}

// Prepare transitions from Open to Prepared, recording the response
// status. Returns an error if the current state is not Open.
func (m *CommitMachine) Prepare(status int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != StateOpen {
		return errors.New("commit: Prepare requires Open state, current: " + m.state.String())
	}
	m.state = StatePrepared
	m.status = status
	return nil
}

// RunHooks transitions from Prepared to HooksRun by executing the
// provided hooks exactly once. All hooks are run even if one fails;
// the first error is returned and causes a transition to Failed
// (strict semantics). If all hooks succeed, the state becomes HooksRun.
//
// This method is idempotent: if hooks have already run, it returns nil
// without re-running them.
func (m *CommitMachine) RunHooks(hooks []func() error) error {
	m.mu.Lock()
	if m.hooksRan {
		m.mu.Unlock()
		return nil
	}
	m.hooksRan = true
	if m.state != StatePrepared && m.state != StateHooksRun {
		m.mu.Unlock()
		return errors.New("commit: RunHooks requires Prepared state, current: " + m.state.String())
	}
	m.mu.Unlock()

	var firstErr error
	for _, h := range hooks {
		if h == nil {
			continue
		}
		if err := h(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if firstErr != nil {
		m.mu.Lock()
		m.state = StateFailed
		m.failErr = firstErr
		m.mu.Unlock()
		return firstErr
	}

	m.mu.Lock()
	m.state = StateHooksRun
	m.mu.Unlock()
	return nil
}

// MarkDurable transitions from HooksRun to Durable, indicating that
// pre-transport persistence and interceptors have succeeded.
func (m *CommitMachine) MarkDurable() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != StateHooksRun {
		return errors.New("commit: MarkDurable requires HooksRun state, current: " + m.state.String())
	}
	m.state = StateDurable
	return nil
}

// Commit transitions from Durable to Committed, recording the final
// status code. The first terminal commit wins; subsequent calls are
// no-ops. Returns an error if the current state is not Durable (or
// already Committed/Observed, in which case nil is returned).
func (m *CommitMachine) Commit(status int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch m.state {
	case StateCommitted, StateObserved:
		// First commit already won; no-op.
		return nil
	case StateDurable:
		m.state = StateCommitted
		m.status = status
		return nil
	default:
		return errors.New("commit: Commit requires Durable state, current: " + m.state.String())
	}
}

// Observe transitions from Committed to Observed, indicating that
// post-commit observers have completed. This is the terminal success state.
func (m *CommitMachine) Observe() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != StateCommitted {
		return errors.New("commit: Observe requires Committed state, current: " + m.state.String())
	}
	m.state = StateObserved
	return nil
}

// Fail transitions to StateFailed, recording the cause error. This is
// only valid from pre-commit states (Open, Prepared, HooksRun, Durable).
// Calling Fail after commit (Committed/Observed) is a no-op that returns
// nil — post-commit failures cannot alter the response and are recorded
// only by the caller via telemetry.
func (m *CommitMachine) Fail(err error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch m.state {
	case StateCommitted, StateObserved:
		// Post-commit failure: cannot alter the response. Caller
		// should record via telemetry. No state change.
		return nil
	case StateFailed:
		// Already failed; keep the first error.
		return nil
	default:
		m.state = StateFailed
		m.failErr = err
		return nil
	}
}
