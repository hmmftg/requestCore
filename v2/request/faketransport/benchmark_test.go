package faketransport

import (
	"testing"
)

func BenchmarkFakeTransportRoundTrip(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ft := New("POST", "/users",
			WithRoutePattern("/users"),
			WithHeader("Content-Type", "application/json"),
			WithBody(`{"name":"alice"}`),
		)
		ft.Context().Response().SetStatus(201)
		ft.Context().Response().SetHeader("Location", "/users/1")
		ft.WriteResponse(ft.Context().Response().Status(), "application/json", []byte(`{"id":"1"}`))
	}
}

func BenchmarkFakeTransportConstruction(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = New("GET", "/users/42",
			WithRoutePattern("/users/{id}"),
			WithPathParam("id", "42"),
			WithQueryParam("page", "1"),
		)
	}
}

func BenchmarkFakeTransportResponseWrite(b *testing.B) {
	body := []byte(`{"status":"ok"}`)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ft := New("GET", "/users")
		ft.WriteResponse(200, "application/json", body)
	}
}
