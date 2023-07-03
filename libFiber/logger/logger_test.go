package logger

import (
	"log"
	"testing"
)

type testLogger struct {
	LogPath     string
	LogSize     int
	LogCompress bool
	SkipPaths   []string
	HeaderName  string
}

func (t testLogger) GetLogPath() string     { return t.LogPath }
func (t testLogger) GetLogSize() int        { return t.LogSize }
func (t testLogger) GetLogCompress() bool   { return t.LogCompress }
func (t testLogger) GetSkipPaths() []string { return t.SkipPaths }
func (t testLogger) GetHeaderName() string  { return t.HeaderName }

func TestFiberLogger(t *testing.T) {
	configs := []testLogger{
		{LogPath: "test1.log", LogSize: 1, LogCompress: true},
		{LogPath: "test2.log", LogSize: 2, LogCompress: false},
		{LogPath: "test3.log", LogSize: 3, LogCompress: true},
	}
	tables := []struct {
		logData   string
		logRepeat int
	}{
		{"589463180000000058946318000000005894631800000000", 100000},
	}

	for _, config := range configs {
		ConfigFiberLogger(config)
		for _, table := range tables {
			for i := 0; i < table.logRepeat; i++ {
				log.Println(table.logData)
			}
		}
	}
}
