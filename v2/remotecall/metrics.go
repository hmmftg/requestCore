package remotecall

import "time"

// MetricsRecorder records metrics for remote calls. Implementations must be
// safe for concurrent use.
type MetricsRecorder interface {
	// RecordCall records a single logical call's metrics.
	RecordCall(opKey string, duration time.Duration, outcome string)
}

// NopRecorder is a MetricsRecorder that discards all metrics.
type NopRecorder struct{}

// RecordCall discards the metrics.
func (NopRecorder) RecordCall(string, time.Duration, string) {}
