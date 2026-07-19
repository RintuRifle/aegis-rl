package limiter

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Strategy selects the rate limiting algorithm executed in Redis.
type Strategy string

const (
	// StrategyTokenBucket is the default: 2 fields per key (tokens, ts).
	StrategyTokenBucket Strategy = "token_bucket"
	// StrategyGCRA stores a single TAT value per key — smaller Redis footprint.
	StrategyGCRA Strategy = "gcra"
)

// Config holds the rate limiting configuration for a tier.
type Config struct {
	Capacity   int64         `json:"capacity"`     // burst size (max tokens)
	RefillRate float64       `json:"refill_rate"`  // tokens per second
	Timeout    time.Duration `json:"timeout"`      // Redis call budget (e.g. 50ms)
}

// Result holds the outcome of a rate limit check.
// Fields ordered largest-to-smallest for optimal struct packing / cache locality.
type Result struct {
	RetryAfter   time.Duration // 8 bytes
	Remaining    int64         // 8 bytes
	Allowed      bool          // 1 byte
	DegradedMode bool          // 1 byte — true if decision came from local fallback
	// 6 bytes padding at struct end only
}

// Limiter is the core rate limiting engine.
// It coordinates between Redis (primary), circuit breaker, and local fallback.
type Limiter struct {
	rdb            *redis.Client
	script         *redis.Script
	strategy       Strategy
	fallback       *LocalFallback
	breaker        *CircuitBreaker
	cfg            Config
	onRedisLatency func(time.Duration) // optional hook for metrics
}

// New creates a new Limiter wired with Redis, circuit breaker, and local fallback.
//
// The Lua script is wrapped in redis.Script, whose Run() tries EVALSHA first
// and transparently falls back to EVAL on NOSCRIPT (e.g. after a Redis restart
// flushes the script cache). Without this, a Redis restart would trip the
// circuit breaker forever even though Redis is healthy.
func New(rdb *redis.Client, strategy Strategy, cfg Config) *Limiter {
	var script *redis.Script
	switch strategy {
	case StrategyGCRA:
		script = redis.NewScript(gcraScript)
	default:
		strategy = StrategyTokenBucket
		script = redis.NewScript(tokenBucketScript)
	}
	return &Limiter{
		rdb:      rdb,
		script:   script,
		strategy: strategy,
		fallback: NewLocalFallback(),
		breaker:  NewCircuitBreaker(5, 10*time.Second), // trip after 5 consecutive fails, 10s cooldown
		cfg:      cfg,
	}
}

// OnRedisLatency registers a hook called with the duration of every Redis
// script call (success or failure). Wire this to a Prometheus histogram.
// Must be called before the limiter starts serving traffic (not goroutine-safe).
func (l *Limiter) OnRedisLatency(f func(time.Duration)) {
	l.onRedisLatency = f
}

// Preload loads the Lua script into the Redis script cache at startup so the
// first real request uses EVALSHA directly. Purely an optimization — Run()
// self-heals on NOSCRIPT either way.
func (l *Limiter) Preload(ctx context.Context) (string, error) {
	return l.script.Load(ctx, l.rdb).Result()
}

// Allow performs the rate-limit check using the limiter's default config.
func (l *Limiter) Allow(ctx context.Context, key string) Result {
	return l.AllowWithConfig(ctx, key, l.cfg)
}

// AllowWithConfig performs the atomic rate-limit check for a given identity key
// using a per-request config (multi-tier / per-endpoint limits).
// Decision flow: circuit breaker check → Redis script → fallback on error.
func (l *Limiter) AllowWithConfig(ctx context.Context, key string, cfg Config) Result {
	// If circuit breaker is open, skip Redis entirely — degrade gracefully
	if l.breaker.Open() {
		return l.fallback.Allow(key, cfg)
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = l.cfg.Timeout
	}
	if timeout <= 0 {
		timeout = 50 * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Run: EVALSHA with automatic EVAL fallback on NOSCRIPT, in one atomic RTT.
	// Timestamps come from Redis TIME inside the script (clock-skew safe).
	start := time.Now()
	res, err := l.script.Run(ctx, l.rdb, []string{key},
		cfg.Capacity, cfg.RefillRate, 1,
	).Result()
	if l.onRedisLatency != nil {
		l.onRedisLatency(time.Since(start))
	}

	if err != nil {
		// Redis call failed — record failure and fall back to local bucket
		l.breaker.RecordFailure()
		return l.fallback.Allow(key, cfg)
	}

	allowed, remaining, retryMs, ok := parseScriptReply(res)
	if !ok {
		// Malformed reply — treat as failure rather than panicking the request path
		l.breaker.RecordFailure()
		return l.fallback.Allow(key, cfg)
	}

	// Redis call succeeded — reset circuit breaker
	l.breaker.RecordSuccess()

	return Result{
		Allowed:    allowed,
		Remaining:  remaining,
		RetryAfter: time.Duration(retryMs) * time.Millisecond,
	}
}

// parseScriptReply safely parses the Lua reply [allowed(0|1), remaining, retry_after_ms]
// without unchecked type assertions (a malformed reply must never panic a request).
func parseScriptReply(res interface{}) (allowed bool, remaining, retryMs int64, ok bool) {
	vals, isSlice := res.([]interface{})
	if !isSlice || len(vals) != 3 {
		return false, 0, 0, false
	}
	var nums [3]int64
	for i, v := range vals {
		n, isInt := v.(int64)
		if !isInt {
			return false, 0, 0, false
		}
		nums[i] = n
	}
	return nums[0] == 1, nums[1], nums[2], true
}

// Strategy returns the active rate limiting strategy.
func (l *Limiter) Strategy() Strategy {
	return l.strategy
}

// CircuitBreakerState returns the current circuit breaker state (for metrics/dashboard).
func (l *Limiter) CircuitBreakerState() CircuitState {
	return l.breaker.State()
}

// GetConfig returns the limiter's default configuration.
func (l *Limiter) GetConfig() Config {
	return l.cfg
}

// StartCleanup starts a background goroutine that periodically cleans up
// expired local fallback buckets to prevent memory leaks.
func (l *Limiter) StartCleanup(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Evict buckets idle for longer than 2x the full refill period
				// (minimum 1 minute so aggressive tiers don't thrash the map)
				maxIdle := time.Minute
				if l.cfg.RefillRate > 0 {
					refillBased := time.Duration(float64(l.cfg.Capacity)/l.cfg.RefillRate*2) * time.Second
					if refillBased > maxIdle {
						maxIdle = refillBased
					}
				}
				l.fallback.Cleanup(maxIdle)
			}
		}
	}()
}
