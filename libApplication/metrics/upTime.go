package metrics

import (
	"time"

	"github.com/Depado/ginprom"
)

func RecordUptime(name string, metric *ginprom.Prometheus) {
	for range time.Tick(time.Second) {
		metric.IncrementCounterValue(name, nil)
	}
}
