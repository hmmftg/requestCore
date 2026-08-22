package handlers

// This file provides persistence helpers and documentation for the v2
// handler lifecycle. The RequestPersister interface is defined in
// request.go; this file adds convenience constructors and a no-op
// implementation for testing.

// NoOpPersister is a RequestPersister that does nothing. It is useful for
// tests and for handlers that do not require request persistence.
type NoOpPersister[Req, Resp any] struct{}

// Insert does nothing and returns nil.
func (NoOpPersister[Req, Resp]) Insert(string, *HandlerRequest[Req, Resp]) error { return nil }

// Update does nothing and returns nil.
func (NoOpPersister[Req, Resp]) Update(string, *HandlerRequest[Req, Resp]) error { return nil }

// NewPersister creates a RequestPersister from insert and update functions.
// Either function may be nil, in which case the corresponding operation is
// a no-op.
func NewPersister[Req, Resp any](
	insert func(path string, trx *HandlerRequest[Req, Resp]) error,
	update func(path string, trx *HandlerRequest[Req, Resp]) error,
) RequestPersister[Req, Resp] {
	return PersisterFunc[Req, Resp]{InsertFn: insert, UpdateFn: update}
}
