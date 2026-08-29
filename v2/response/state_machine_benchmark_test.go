package response

import (
	"testing"
)

func BenchmarkCommitMachineFullCycle(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := NewCommitMachine()
		_ = m.Prepare(200)
		_ = m.RunHooks(nil)
		_ = m.MarkDurable()
		_ = m.Commit(200)
		_ = m.Observe()
	}
}

func BenchmarkCommitMachineWithHooks(b *testing.B) {
	hooks := []func() error{
		func() error { return nil },
		func() error { return nil },
		func() error { return nil },
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := NewCommitMachine()
		_ = m.Prepare(200)
		_ = m.RunHooks(hooks)
		_ = m.MarkDurable()
		_ = m.Commit(200)
		_ = m.Observe()
	}
}

func BenchmarkCommitMachineConcurrentCommit(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := NewCommitMachine()
		_ = m.Prepare(200)
		_ = m.RunHooks(nil)
		_ = m.MarkDurable()
		// Single-goroutine commit (concurrent benchmark is in the
		// race test; this measures the non-contended path).
		_ = m.Commit(200)
	}
}

func BenchmarkCommitMachineStateCheck(b *testing.B) {
	m := NewCommitMachine()
	_ = m.Prepare(200)
	_ = m.RunHooks(nil)
	_ = m.MarkDurable()
	_ = m.Commit(200)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Committed()
		_ = m.State()
		_ = m.Status()
	}
}

func BenchmarkProblemConstruction(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = NewProblem(422, "Validation Failed").
			WithDetail("email is required").
			WithCode("VALIDATION_ERROR").
			WithRequestID("req-123").
			WithTraceID("trace-456")
	}
}

func BenchmarkProblemMarshalJSON(b *testing.B) {
	p := NewProblem(422, "Validation Failed").
		WithDetail("email is required").
		WithCode("VALIDATION_ERROR").
		WithRequestID("req-123").
		WithTraceID("trace-456")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = p.MarshalJSON()
	}
}

func BenchmarkProblemWithViolationsMarshal(b *testing.B) {
	p := NewValidationProblem(422, "Validation Failed", []Violation{
		{Field: "email", Rule: "required", Message: "email is required"},
		{Field: "age", Rule: "min", Message: "age must be at least 18"},
		{Field: "name", Rule: "max_length", Message: "name too long"},
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = p.MarshalJSON()
	}
}

func BenchmarkMapperRegistryMap(b *testing.B) {
	r := DefaultMapperRegistry()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.Map(nil)
	}
}
