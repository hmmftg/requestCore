// Package operation provides immutable endpoint metadata and a
// registry contract shared by handlers, OpenAPI generation, policies,
// and the application bootstrap.
//
// An Operation captures the static, registration-time metadata for a
// single endpoint: operation ID, HTTP method, route pattern, name,
// summary, description, tags, and deprecation status. Dynamic runtime
// state (status overrides, dynamic headers) is not stored here.
//
// operation imports only the Go standard library. It does not import
// handlers, app, request, or any v1 package.
package operation

import "fmt"

// Operation is the immutable metadata for a single endpoint, captured
// at registration time. It is shared by handlers, OpenAPI generation,
// policy enforcement, and the application bootstrap.
//
// Once created, an Operation should not be mutated. The Registry
// stores copies and returns copies to prevent external mutation.
type Operation struct {
	// ID is the unique operation identifier (e.g. "createUser").
	// Duplicate IDs fail registration.
	ID string

	// Method is the HTTP method (GET, POST, PUT, PATCH, DELETE, HEAD).
	Method string

	// Pattern is the canonical route pattern using {param} syntax
	// (e.g. "/users/{id}").
	Pattern string

	// Name is a short human-readable name for the operation.
	Name string

	// Summary is a one-line summary for OpenAPI documentation.
	Summary string

	// Description is a longer description for OpenAPI documentation.
	Description string

	// Tags group operations for OpenAPI documentation.
	Tags []string

	// Deprecated marks the operation as deprecated in OpenAPI output.
	Deprecated bool
}

// NewOperation creates an Operation with the required ID, method, and
// pattern. Optional fields can be set via builder methods.
func NewOperation(id, method, pattern string) Operation {
	return Operation{
		ID:      id,
		Method:  method,
		Pattern: pattern,
	}
}

// WithName sets the human-readable name and returns the operation.
func (o Operation) WithName(name string) Operation {
	o.Name = name
	return o
}

// WithSummary sets the summary and returns the operation.
func (o Operation) WithSummary(summary string) Operation {
	o.Summary = summary
	return o
}

// WithDescription sets the description and returns the operation.
func (o Operation) WithDescription(desc string) Operation {
	o.Description = desc
	return o
}

// WithTags sets the tags and returns the operation.
func (o Operation) WithTags(tags ...string) Operation {
	o.Tags = append([]string(nil), tags...)
	return o
}

// WithDeprecated marks the operation as deprecated and returns it.
func (o Operation) WithDeprecated() Operation {
	o.Deprecated = true
	return o
}

// String returns a diagnostic string representation.
func (o Operation) String() string {
	return fmt.Sprintf("%s %s [%s]", o.Method, o.Pattern, o.ID)
}

// Clone returns a deep copy of the operation, including the tags slice.
func (o Operation) Clone() Operation {
	cp := o
	if o.Tags != nil {
		cp.Tags = append([]string(nil), o.Tags...)
	}
	return cp
}
