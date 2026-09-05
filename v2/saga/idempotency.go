package saga

import "fmt"

// IdempotencyKey is a unique identifier for a step execution or
// compensation. It allows external systems to deduplicate operations
// when a saga is retried or resumed after a crash.
//
// Format: "<sagaID>:<stepName>:<phase>" where phase is "execute" or
// "compensate".
type IdempotencyKey string

// MakeIdempotencyKey constructs an idempotency key from the saga ID,
// step name, and phase.
func MakeIdempotencyKey(sagaID, stepName, phase string) IdempotencyKey {
	return IdempotencyKey(fmt.Sprintf("%s:%s:%s", sagaID, stepName, phase))
}

// String returns the string representation of the idempotency key.
func (k IdempotencyKey) String() string {
	return string(k)
}

// IsStepExecuted returns true if the step with the given name has
// completed execution (status == StepDone).
func (st *SagaState) IsStepExecuted(stepName string) bool {
	for i := range st.Steps {
		if st.Steps[i].Name == stepName {
			return st.Steps[i].Status == StepDone
		}
	}
	return false
}

// IsStepCompensated returns true if the step with the given name has
// completed compensation (status == StepCompensated).
func (st *SagaState) IsStepCompensated(stepName string) bool {
	for i := range st.Steps {
		if st.Steps[i].Name == stepName {
			return st.Steps[i].Status == StepCompensated
		}
	}
	return false
}

// StepIndex returns the index of the step with the given name, or -1
// if not found.
func (st *SagaState) StepIndex(stepName string) int {
	for i := range st.Steps {
		if st.Steps[i].Name == stepName {
			return i
		}
	}
	return -1
}
