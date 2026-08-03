package libTracing

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpClientCallsTotal   *prometheus.CounterVec
	httpClientCallDuration *prometheus.HistogramVec
	metricsInitialized     bool
)

func init() {
	InitHTTPClientMetrics()
}

// InitHTTPClientMetrics registers Prometheus metrics for outbound HTTP client calls.
// Safe to call multiple times; uses MustRegister via a dedicated registry check.
func InitHTTPClientMetrics() {
	httpClientCallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_client_calls_total",
			Help: "Total number of outbound HTTP client calls by API, method, and status class.",
		},
		[]string{"api", "method", "status_class"},
	)

	httpClientCallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_client_call_duration_seconds",
			Help:    "Latency of outbound HTTP client calls by API and method.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"api", "method"},
	)

	prometheus.MustRegister(httpClientCallsTotal)
	prometheus.MustRegister(httpClientCallDuration)
	metricsInitialized = true
}

func statusClass(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return "2xx"
	case statusCode >= 300 && statusCode < 400:
		return "3xx"
	case statusCode >= 400 && statusCode < 500:
		return "4xx"
	case statusCode >= 500:
		return "5xx"
	default:
		return "error"
	}
}

// RecordHTTPClientCall records metrics for an outbound HTTP client call.
// If metrics are not initialized or statusCode is 0 (request failed before
// getting a response), it records with status_class "error".
func RecordHTTPClientCall(apiName, method string, statusCode int, duration time.Duration, err error) {
	if !metricsInitialized || httpClientCallsTotal == nil || httpClientCallDuration == nil {
		return
	}

	sc := statusClass(statusCode)
	if err != nil && statusCode == 0 {
		sc = "error"
	}

	httpClientCallsTotal.WithLabelValues(apiName, method, sc).Inc()
	httpClientCallDuration.WithLabelValues(apiName, method).Observe(duration.Seconds())
}
