package operation

import (
	"errors"
	"sort"
	"sync"
)

// Sentinel errors for registry operations.
var (
	// ErrDuplicateOperationID is returned when an operation with the
	// same ID is already registered.
	ErrDuplicateOperationID = errors.New("operation: duplicate operation ID")

	// ErrRegistryFrozen is returned when attempting to register or
	// modify a frozen registry.
	ErrRegistryFrozen = errors.New("operation: registry is frozen")

	// ErrInvalidOperation is returned when an operation has an empty ID,
	// method, or pattern.
	ErrInvalidOperation = errors.New("operation: invalid operation (empty ID, method, or pattern)")
)

// Registry is the interface for storing and retrieving operations.
// It is mutable during startup and frozen before serving.
type Registry interface {
	// Register adds an operation to the registry. Returns
	// ErrDuplicateOperationID if an operation with the same ID exists,
	// ErrRegistryFrozen if the registry is frozen, or ErrInvalidOperation
	// if the operation has empty required fields.
	Register(op Operation) error

	// Get retrieves an operation by its ID. Returns the operation and
	// true if found, or a zero Operation and false if not found.
	Get(id string) (Operation, bool)

	// All returns all registered operations sorted by ID. The returned
	// slice is a copy and may be safely mutated by the caller.
	All() []Operation

	// Freeze prevents further registration. Called after startup.
	Freeze()

	// Frozen reports whether the registry is frozen.
	Frozen() bool
}

// DefaultRegistry is the default Registry implementation. It is safe
// for concurrent use during registration and lookup.
type DefaultRegistry struct {
	mu       sync.RWMutex
	ops      map[string]Operation
	frozen   bool
}

// NewRegistry creates a new empty DefaultRegistry.
func NewRegistry() *DefaultRegistry {
	return &DefaultRegistry{
		ops: make(map[string]Operation),
	}
}

// Register adds an operation to the registry.
func (r *DefaultRegistry) Register(op Operation) error {
	if op.ID == "" || op.Method == "" || op.Pattern == "" {
		return ErrInvalidOperation
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrRegistryFrozen
	}
	if _, exists := r.ops[op.ID]; exists {
		return ErrDuplicateOperationID
	}
	r.ops[op.ID] = op.Clone()
	return nil
}

// Get retrieves an operation by its ID.
func (r *DefaultRegistry) Get(id string) (Operation, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	op, ok := r.ops[id]
	if !ok {
		return Operation{}, false
	}
	return op.Clone(), true
}

// All returns all registered operations sorted by ID.
func (r *DefaultRegistry) All() []Operation {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.ops))
	for id := range r.ops {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]Operation, 0, len(ids))
	for _, id := range ids {
		result = append(result, r.ops[id].Clone())
	}
	return result
}

// Freeze prevents further registration.
func (r *DefaultRegistry) Freeze() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frozen = true
}

// Frozen reports whether the registry is frozen.
func (r *DefaultRegistry) Frozen() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.frozen
}
