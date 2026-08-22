package workers

import (
	"math/rand"
	"runtime"
	"sync"
	"time"
)

func getNumCPU() int {
	return runtime.NumCPU()
}

// globalRand is a package-level locked random source for default jitter.
var globalRand struct {
	mu  sync.Mutex
	rng *rand.Rand
}

func init() {
	globalRand.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

// defaultJitter returns a random int64 in [0, max) using the package-level
// locked random source.
func defaultJitter(max int64) int64 {
	if max <= 0 {
		return 0
	}
	globalRand.mu.Lock()
	defer globalRand.mu.Unlock()
	return globalRand.rng.Int63n(max)
}

// calculateBackoff computes the retry delay with optional jitter.
// The backoff doubles with each attempt, capped at maxBackoff.
// The jitterSource function is used for deterministic testing.
func calculateBackoff(initial, max time.Duration, attempt int, jitter bool, jitterSource func(int64) int64) time.Duration {
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
		jitterAmount := time.Duration(jitterSource(int64(backoff) / 2))
		backoff += jitterAmount
		if backoff > max {
			backoff = max
		}
	}
	return backoff
}
