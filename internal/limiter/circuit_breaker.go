package limiter

import (
	"sync"
	"time"
)

// CircuitState represents the current state of the circuit breaker.
type CircuitState int

const (
	// StateClosed is the normal operating state — requests go to Redis.
	StateClosed CircuitState = iota
	// StateOpen means Redis is presumed down — all requests go to local fallback.
	StateOpen
	// StateHalfOpen allows a single probe request to Redis to check recovery.
	StateHalfOpen
)

// CircuitBreaker wraps Redis calls to prevent cascading failures.
// After N consecutive failures, it trips open and routes to the local fallback.
// After a cooldown period, it enters half-open state and allows exactly ONE
// probe request through — every other concurrent request keeps using the
// fallback until the probe reports back. Without this single-probe guard,
// a busy server would slam a still-down Redis with thousands of requests
// the moment the cooldown expires, each paying the full timeout budget.
type CircuitBreaker struct {
	mu               sync.Mutex
	state            CircuitState
	consecutiveFails int
	maxFails         int
	cooldown         time.Duration
	lastFailTime     time.Time
	probing          bool // true while a half-open probe is in flight
}

// NewCircuitBreaker creates a new circuit breaker.
//   - maxFails: number of consecutive failures before tripping open
//   - cooldown: time to wait before probing Redis again after tripping
func NewCircuitBreaker(maxFails int, cooldown time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:    StateClosed,
		maxFails: maxFails,
		cooldown: cooldown,
	}
}

// Open returns true if the caller should skip Redis and use the local fallback.
// It also transitions Open → HalfOpen after the cooldown expires, electing the
// calling request as the single probe.
func (cb *CircuitBreaker) Open() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return false
	case StateOpen:
		// Check if cooldown has elapsed — if so, this request becomes the probe
		if time.Since(cb.lastFailTime) > cb.cooldown {
			cb.state = StateHalfOpen
			cb.probing = true
			return false // allow the probe request through to Redis
		}
		return true // still within cooldown, skip Redis
	case StateHalfOpen:
		if cb.probing {
			return true // a probe is already in flight — everyone else falls back
		}
		cb.probing = true
		return false // become the probe
	}
	return false
}

// RecordSuccess records a successful Redis call.
// If in HalfOpen state, transitions back to Closed (Redis recovered).
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFails = 0
	cb.probing = false
	cb.state = StateClosed
}

// RecordFailure records a failed Redis call.
// A failed half-open probe re-opens the breaker immediately; in closed state,
// consecutive failures past the threshold trip it open.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFails++
	cb.lastFailTime = time.Now()

	if cb.state == StateHalfOpen || cb.consecutiveFails >= cb.maxFails {
		cb.state = StateOpen
	}
	cb.probing = false
}

// State returns the current circuit breaker state (for metrics/logging).
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// String returns a human-readable state name.
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}
