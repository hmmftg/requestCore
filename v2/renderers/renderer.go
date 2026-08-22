// Package renderers provides pluggable content renderers for v2 response handling.
//
// A Renderer encodes data into a byte payload and declares its content type.
// Framework adapters are responsible for writing the bytes to their specific
// transport (net/http.ResponseWriter, fasthttp.Response, etc.).
//
// Built-in renderers: JSON, XML, Text, CSV.
package renderers

// Renderer encodes data into a byte payload with a declared content type.
// Implementations must be safe for concurrent use.
type Renderer interface {
	// ContentType returns the MIME content type for this renderer.
	ContentType() string

	// Encode serializes data into a byte payload.
	// Returns an error if encoding fails; the caller is responsible
	// for routing the error through the error handler registry.
	Encode(data any) ([]byte, error)
}
