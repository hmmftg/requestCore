// Package nextadapter: log_value.go previously contained safeLogValue,
// which masked sensitive fields in typed responses for the AddLog
// pipeline. This is no longer needed because the executor now uses
// SlogSink, which handles LogValuer masking via the telemetry contract.
package nextadapter
