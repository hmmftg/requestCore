package saga

import "sync"

// SagaRegistry maps saga names to their step definitions (Saga with
// real Execute/Compensate functions). It is used by ResumeAll to
// reconstruct sagas with real step functions from persisted state.
//
// Applications register their saga definitions at startup, then
// ResumeAll can look up the definition by SagaName and merge it with
// the persisted StepState to resume execution with real functions.
type SagaRegistry struct {
	mu    sync.RWMutex
	sagas map[string]*Saga
}

// NewSagaRegistry creates a new empty registry.
func NewSagaRegistry() *SagaRegistry {
	return &SagaRegistry{
		sagas: make(map[string]*Saga),
	}
}

// Register adds a saga definition to the registry. The saga Name is
// used as the key. Returns false if a saga with the same name is
// already registered.
func (r *SagaRegistry) Register(saga *Saga) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.sagas[saga.Name]; exists {
		return false
	}
	r.sagas[saga.Name] = saga
	return true
}

// Lookup retrieves a saga definition by name. Returns nil if not found.
func (r *SagaRegistry) Lookup(name string) *Saga {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sagas[name]
}
