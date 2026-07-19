package limiter

import (
	"sync"
	"time"
)

// localBucket represents an in-process token bucket for a single key.
// Used as degraded-mode fallback when Redis is unreachable.
type localBucket struct {
	tokens float64
	lastTS time.Time
	mu     sync.Mutex
}

// LocalFallback provides per-instance, in-process rate limiting using sync.Map.
// Less accurate than Redis (no cross-instance coordination) but keeps the API
// available in a degraded mode rather than fail-closed.
type LocalFallback struct {
	buckets sync.Map // map[string]*localBucket
}

// NewLocalFallback creates a new in-process fallback limiter.
func NewLocalFallback() *LocalFallback {
	return &LocalFallback{}
}

// Allow checks the local token bucket for the given key.
// Same token bucket math as the Lua script, but entirely in-process.
func (lf *LocalFallback) Allow(key string, cfg Config) Result {
	now := time.Now()

	// Load or create bucket for this key
	val, _ := lf.buckets.LoadOrStore(key, &localBucket{
		tokens: float64(cfg.Capacity),
		lastTS: now,
	})
	b := val.(*localBucket)

	b.mu.Lock()
	defer b.mu.Unlock()

	// Refill tokens based on elapsed time
	elapsed := now.Sub(b.lastTS).Seconds()
	b.tokens = min64(float64(cfg.Capacity), b.tokens+elapsed*cfg.RefillRate)
	b.lastTS = now

	// Check if request is allowed
	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return Result{
			Allowed:      true,
			Remaining:    int64(b.tokens),
			RetryAfter:   0,
			DegradedMode: true,
		}
	}

	// Denied — calculate retry-after
	retryAfter := time.Duration((1.0-b.tokens)/cfg.RefillRate*1000) * time.Millisecond

	return Result{
		Allowed:      false,
		Remaining:    0,
		RetryAfter:   retryAfter,
		DegradedMode: true,
	}
}

// Cleanup removes expired buckets to prevent memory leaks.
// Should be called periodically from a background goroutine.
func (lf *LocalFallback) Cleanup(maxIdle time.Duration) {
	now := time.Now()
	lf.buckets.Range(func(key, value any) bool {
		b := value.(*localBucket)
		b.mu.Lock()
		idle := now.Sub(b.lastTS)
		b.mu.Unlock()
		if idle > maxIdle {
			lf.buckets.Delete(key)
		}
		return true
	})
}

func min64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
