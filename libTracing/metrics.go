package libTracing

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// HTTPClientMetricsRecorder is the interface for recording outbound HTTP client call metrics.
// Inject a custom implementation for testing; use the default Prometheus-backed recorder in production.
type HTTPClientMetricsRecorder interface {
	Record(api, method string, statusCode int, duration time.Duration, outcome string)
}

var (
	httpClientCallsTotal   *prometheus.CounterVec
	httpClientCallDuration *prometheus.HistogramVec
	metricsInitialized     bool
	initOnce               sync.Once

	defaultRecorder HTTPClientMetricsRecorder
)

func init() {
	InitHTTPClientMetrics()
}

// InitHTTPClientMetrics registers Prometheus metrics for outbound HTTP client calls.
// Safe to call multiple times; uses sync.Once to prevent duplicate-registration panics.
func InitHTTPClientMetrics() {
	initOnce.Do(func() {
		httpClientCallsTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_client_calls_total",
				Help: "Total number of outbound HTTP client calls by API, method, status class, and outcome.",
			},
			[]string{"api", "method", "status_class", "outcome"},
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
		defaultRecorder = &prometheusRecorder{}
	})
}

// prometheusRecorder is the default HTTPClientMetricsRecorder backed by Prometheus.
type prometheusRecorder struct{}

func (r *prometheusRecorder) Record(api, method string, statusCode int, duration time.Duration, outcome string) {
	if !metricsInitialized || httpClientCallsTotal == nil || httpClientCallDuration == nil {
		return
	}
	sc := statusClass(statusCode)
	httpClientCallsTotal.WithLabelValues(api, method, sc, outcome).Inc()
	httpClientCallDuration.WithLabelValues(api, method).Observe(duration.Seconds())
}

// DefaultHTTPClientMetricsRecorder returns the default Prometheus-backed recorder.
func DefaultHTTPClientMetricsRecorder() HTTPClientMetricsRecorder {
	InitHTTPClientMetrics()
	return defaultRecorder
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
// This is a backward-compatible wrapper that derives outcome from err.
// Use RecordHTTPClientCallWithOutcome for explicit outcome control (e.g. "timeout").
func RecordHTTPClientCall(apiName, method string, statusCode int, duration time.Duration, err error) {
	outcome := "success"
	if err != nil {
		outcome = "failure"
	}
	RecordHTTPClientCallWithOutcome(apiName, method, statusCode, duration, err, outcome)
}

// RecordHTTPClientCallWithOutcome records metrics with an explicit outcome label
// (success, failure, timeout) in addition to the HTTP status class.
func RecordHTTPClientCallWithOutcome(apiName, method string, statusCode int, duration time.Duration, _ error, outcome string) {
	DefaultHTTPClientMetricsRecorder().Record(apiName, method, statusCode, duration, outcome)
}
