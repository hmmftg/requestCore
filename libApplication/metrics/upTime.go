// Package metrics provides Prometheus metrics collection for requestCore applications.
package metrics

import (
	"log"
	"time"

	"github.com/Depado/ginprom"
)

// RecordUptime increments the named Prometheus counter every second to track application uptime.
func RecordUptime(name string, metric *ginprom.Prometheus) {
	var err error
	for range time.Tick(time.Second) {
		err = metric.IncrementCounterValue(name, nil)
		if err != nil {
			log.Println("error in increment uptime", err)
		}
	}
}
