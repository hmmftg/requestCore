package libParams

import (
	"fmt"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// SplunkParams holds configuration for the Splunk logger.
type SplunkParams struct {
	URL        string `yaml:"url"`        // Splunk HEC endpoint (e.g., "https://<splunk-server>:8088/services/collector/event")
	Token      string `yaml:"token"`      // HEC token
	SourceType string `yaml:"source"`     // Splunk source type
	Source     string `yaml:"sourceType"` // Splunk source
	Index      string `yaml:"index"`      // Splunk index
	RawHandler bool   `yaml:"rawHandler"`
}

// Logging related params
type LogParams struct {
	LogPath     string        `yaml:"logPath"`
	LogSize     int           `yaml:"logSize"`
	LogCompress bool          `yaml:"logCompress"`
	SkipPaths   []string      `yaml:"skipPaths"`
	LogHeader   string        `yaml:"logHeader"`
	UseSlog     bool          `yaml:"useSlog"`
	Splunk      *SplunkParams `yaml:"splunkParams"`
}

func (m ApplicationParams[SpecialParams]) GetLogging() LogParams {
	return m.Logging
}

func LogRotate(logger *lumberjack.Logger) {
	// ticker := time.NewTicker(time.Second * 1)
	ticker := time.NewTicker(time.Hour)
	go func() {
		for {
			<-ticker.C
			h, m, s := time.Now().Clock()
			if h == 0 {
				//if m == 0 && h == 0 {
				fmt.Println("========= performe log-rotate", h, m, s)
				err := logger.Rotate()
				if err != nil {
					fmt.Println("========= error in log-rotate", err)
				}
			} else {
				fmt.Println("log-heart-beat", h, m, s)
			}
		}
	}()
}
