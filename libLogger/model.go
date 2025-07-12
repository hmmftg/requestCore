package libLogger

type LoggerInterface interface {
	GetLogPath() string
	GetLogSize() int
	GetLogCompress() bool
	GetSkipPaths() []string
	GetHeaderName() string
}
