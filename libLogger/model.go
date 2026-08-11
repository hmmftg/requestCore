package libLogger

// LoggerInterface defines the interface for accessing logging configuration parameters.
type LoggerInterface interface {
	GetLogPath() string
	GetLogSize() int
	GetLogCompress() bool
	GetSkipPaths() []string
	GetHeaderName() string
}
