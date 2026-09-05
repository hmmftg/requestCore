package saga

// deepCopySagaState creates a deep copy of SagaState so that mutations
// to the original (or to slices/maps inside it) do not affect the copy.
func deepCopySagaState(st *SagaState) *SagaState {
	cp := *st

	if st.Steps != nil {
		cp.Steps = make([]StepState, len(st.Steps))
		for i := range st.Steps {
			cp.Steps[i] = st.Steps[i]
		}
	}

	if st.Data != nil {
		cp.Data = make(map[string]any, len(st.Data))
		for k, v := range st.Data {
			cp.Data[k] = v
		}
	}

	cp.outboxBuffer = nil

	return &cp
}
