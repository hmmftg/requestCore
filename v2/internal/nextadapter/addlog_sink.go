// Package nextadapter: addlog_sink.go previously contained addLogSink,
// which forwarded telemetry events to the legacy AddLog pipeline. This
// is no longer needed because the executor now defaults to SlogSink
// (the v2 telemetry path with verified Splunk ingestion).
package nextadapter
