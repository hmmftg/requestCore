package remotecall

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// NewInstrumentedClient creates a standard *http.Client with OpenTelemetry
// transport instrumentation. This is used when no client is injected via
// WithHTTPClient.
//
// If you inject a client via WithHTTPClient, you own transport
// instrumentation. requestCore does NOT add otelhttp. No transport-type
// introspection is performed.
func NewInstrumentedClient() *http.Client {
	return &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
}
