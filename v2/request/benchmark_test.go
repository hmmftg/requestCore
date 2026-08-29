package request

import (
	"context"
	"net/http"
	"testing"
)

func BenchmarkRequestContextCreation(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = NewContext(context.Background(),
			WithMethod("POST"),
			WithPath("/users/42"),
			WithRoutePattern("/users/{id}"),
		)
	}
}

func BenchmarkTypedKeySetGet(b *testing.B) {
	ctx := NewContext(context.Background())
	k := NewKey[string]("test-key")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Set(ctx, k, "value")
		_, _ = Get(ctx, k)
	}
}

func BenchmarkTypedKeyGetMissing(b *testing.B) {
	ctx := NewContext(context.Background())
	k := NewKey[string]("missing-key")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Get(ctx, k)
	}
}

func BenchmarkResponseStateMutation(b *testing.B) {
	r := NewResponseState()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.SetStatus(201)
		r.SetHeader("Location", "/users/1")
		_ = r.Status()
		_ = r.Header()
	}
}

func BenchmarkResponseStateClone(b *testing.B) {
	r := NewResponseState()
	r.SetStatus(201)
	r.SetHeader("Location", "/users/1")
	r.AddHeader("X-Custom", "a")
	r.AddHeader("X-Custom", "b")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.Clone()
	}
}

func BenchmarkContextHeaderAccess(b *testing.B) {
	h := make(http.Header)
	h.Set("Authorization", "Bearer token")
	h.Set("Content-Type", "application/json")
	h.Set("X-Request-ID", "req-123")
	ctx := NewContext(context.Background(), WithHeader(h))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ctx.Header("Authorization")
	}
}

func BenchmarkContextPathParams(b *testing.B) {
	params := map[string]string{"id": "42", "org": "acme"}
	ctx := NewContext(context.Background(), WithPathParams(params))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ctx.PathParam("id")
	}
}

func BenchmarkContextWithOptions(b *testing.B) {
	h := make(http.Header)
	h.Set("Authorization", "Bearer token")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewContext(context.Background(),
			WithMethod("POST"),
			WithPath("/users/42"),
			WithRoutePattern("/users/{id}"),
			WithHeader(h),
			WithPathParams(map[string]string{"id": "42"}),
			WithRemoteAddr("127.0.0.1:1234"),
		)
	}
}
