package splunk

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/hmmftg/requestCore/libParams"
)

var httpClient = &http.Client{
	Timeout: 2 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	},
}

// SplunkLogger is a logger that sends logs to Splunk.
type SplunkLogger struct {
	lock   *sync.Mutex
	params *libParams.SplunkParams
}

// SplunkLog represents the structure of a log event sent to Splunk.
type SplunkLog struct {
	Event      string `json:"event"`
	SourceType string `json:"sourcetype"`
	Source     string `json:"-"`
	Index      string `json:"-"`
}

func (j SplunkLogger) write(p []byte) (n int, err error) {
	j.lock.Lock()
	defer j.lock.Unlock()
	event := string(p)
	if p[len(p)-1] == '\n' {
		event = string(p[:len(p)-1])
	}

	// Create the Splunk log event
	js := SplunkLog{
		Event:      event,
		SourceType: j.params.SourceType,
		Source:     j.params.Source,
		Index:      j.params.Index,
	}

	// Marshal the log event to JSON
	buffer, err := json.Marshal(js)
	if err != nil {
		log.Printf("Failed to marshal Splunk log event: %v", err)
		return 0, err
	}

	// Create the HTTP request
	req, err := http.NewRequest("POST", j.params.URL, bytes.NewReader(buffer))
	if err != nil {
		log.Printf("Failed to create Splunk request: %v", err)
		return 0, err
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("Splunk %s", j.params.Token))
	req.Header.Set("Content-Type", "application/json")

	// Send the request using a custom HTTP client with a timeout
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("Failed to send Splunk request: %v", err)
		return 0, err
	}
	defer resp.Body.Close() // Always close the response body

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		log.Printf("Splunk request failed with status: %v", resp.Status)
		return 0, fmt.Errorf("Splunk request failed with status: %v", resp.Status)
	}
	return len(p), nil
}

// Write sends a log message to Splunk async.
func (j SplunkLogger) Write(p []byte) (n int, err error) {
	return j.write(p)
	// go j.writeWrapper(p)
	// return len(p), nil
}

// checkIfSplunkIsWorking tests the Splunk connection by sending a test log.
func CheckIfSplunkIsWorking(params *libParams.SplunkParams) (*SplunkLogger, error) {
	logger := SplunkLogger{
		lock:   &sync.Mutex{},
		params: params,
	}
	_, err := logger.write([]byte("Startup"))
	if err != nil {
		return nil, err
	}
	return &logger, nil
}

type LogSplunk struct {
	Logger     string `json:"logger"`
	Severity   string `json:"serverity"`
	Header     string `json:"header,omitempty"`
	Status     int    `json:"status"`
	Client     string `json:"client"`
	Latency    string `json:"latency"`
	LatencyInt int64  `json:"latencyMicro"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Error      string `json:"error,omitempty"`
}
