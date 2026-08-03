package libCallApi

import (
	"crypto/tls"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// NewInstrumentedHTTPClient creates a dedicated *http.Client with OpenTelemetry
// transport instrumentation. Each call returns an independent client with its
// own transport; the client is safe for concurrent use.
//
// Parameters:
//   - timeout: client-level timeout. Zero means no client-level timeout
//     (the caller is responsible for cancellation via context).
//   - skipTLS: when true, TLS certificate verification is disabled.
//     Defaults to false (verification enabled) for production safety.
//
// The returned client wraps its transport with otelhttp.NewTransport so that
// outbound HTTP spans are automatically created for requests made through it.
// This complements—but does not duplicate—the manual span creation in
// ConsumeRestJSON, because otelhttp.NewTransport only creates child spans
// when a parent span is already active in the context.
func NewInstrumentedHTTPClient(timeout time.Duration, skipTLS bool) *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipTLS,
		},
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: otelhttp.NewTransport(transport),
	}
}
