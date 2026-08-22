package workers

import (
	"math/rand"
	"runtime"
	"time"
)

func getNumCPU() int {
	return runtime.NumCPU()
}

// calculateBackoff computes the retry delay with optional jitter.
// The backoff doubles with each attempt, capped at maxBackoff.
func calculateBackoff(initial, max time.Duration, attempt int, jitter bool) time.Duration {
	backoff := initial
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff > max {
			backoff = max
			break
		}
	}
	if jitter {
		// Add up to 50% jitter
		jitterAmount := time.Duration(rand.Int63n(int64(backoff) / 2))
		backoff += jitterAmount
		if backoff > max {
			backoff = max
		}
	}
	return backoff
}
