package libNetHttp

import "testing"

func TestNetHTTPPathMatches(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{"exact", "/users", "/users", true},
		{"exact mismatch", "/users", "/posts", false},
		{"single wildcard", "/users/{id}", "/users/42", true},
		{"single wildcard mismatch", "/users/{id}", "/users/42/posts", false},
		{"multi wildcard", "/files/{path...}", "/files/a/b/c", true},
		{"multi wildcard one segment", "/files/{path...}", "/files/a", true},
		{"multi wildcard zero segments", "/files/{path...}", "/files/", true},
		{"method prefix stripped", "GET /users/{id}", "/users/42", true},
		{"different path length", "/users/{id}", "/users", false},
		{"root", "/", "/", true},
		{"trailing slash", "/users/", "/users/", true},
		{"trailing slash mismatch", "/users/", "/users", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := netHTTPPathMatches(tt.pattern, tt.path)
			if got != tt.want {
				t.Fatalf("netHTTPPathMatches(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

func TestPathMatchesDifferentMethod(t *testing.T) {
	routes := []routeEntry{
		{method: "GET", pattern: "/users/{id}"},
		{method: "POST", pattern: "/users"},
	}
	tests := []struct {
		name   string
		path   string
		method string
		want   bool
	}{
		{"different method", "/users/42", "DELETE", true},
		{"same method", "/users/42", "GET", false},
		{"different method root", "/users", "PUT", true},
		{"unmatched path", "/posts", "GET", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pathMatchesDifferentMethod(routes, tt.path, tt.method)
			if got != tt.want {
				t.Fatalf("pathMatchesDifferentMethod(%q, %q) = %v, want %v", tt.path, tt.method, got, tt.want)
			}
		})
	}
}
