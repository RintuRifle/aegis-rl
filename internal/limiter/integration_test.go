package limiter

// Integration tests that exercise the REAL Lua scripts against a live Redis.
// They skip automatically when Redis is unreachable (set REDIS_ADDR to point
// at one; CI provides a redis:7 service container so these always run there).

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr, DialTimeout: 500 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis not available at %s: %v", addr, err)
	}
	return rdb
}

func uniqueKey(prefix string) string {
	return fmt.Sprintf("%s:%d", prefix, time.Now().UnixNano())
}

// TestIntegration_TokenBucket_Atomicity is the proof-of-atomicity stress test:
// N concurrent goroutines hammer the SAME key; the number of allowed requests
// must exactly equal the bucket capacity — any read-modify-write race would
// let extras through.
func TestIntegration_TokenBucket_Atomicity(t *testing.T) {
	rdb := testRedis(t)
	defer rdb.Close()

	const capacity = 50
	const attempts = 500

	lim := New(rdb, StrategyTokenBucket, Config{
		Capacity:   capacity,
		RefillRate: 0.001, // effectively no refill during the test window
		Timeout:    2 * time.Second,
	})

	key := uniqueKey("it:tb")
	var allowed atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := lim.Allow(context.Background(), key)
			if res.DegradedMode {
				t.Error("should not degrade with healthy redis")
			}
			if res.Allowed {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := allowed.Load(); got != capacity {
		t.Errorf("atomicity violated: %d allowed, want exactly %d", got, capacity)
	}
}

// TestIntegration_NoscriptRecovery simulates a Redis restart (SCRIPT FLUSH
// clears the script cache) — the limiter must transparently recover via
// EVAL instead of tripping the circuit breaker.
func TestIntegration_NoscriptRecovery(t *testing.T) {
	rdb := testRedis(t)
	defer rdb.Close()

	lim := New(rdb, StrategyTokenBucket, Config{
		Capacity: 10, RefillRate: 1, Timeout: 2 * time.Second,
	})

	ctx := context.Background()
	if _, err := lim.Preload(ctx); err != nil {
		t.Fatalf("preload failed: %v", err)
	}

	// First call works via EVALSHA
	if res := lim.Allow(ctx, uniqueKey("it:ns")); !res.Allowed || res.DegradedMode {
		t.Fatalf("first call should be allowed via redis, got %+v", res)
	}

	// Simulate restart: flush the script cache
	if err := rdb.ScriptFlush(ctx).Err(); err != nil {
		t.Fatalf("script flush failed: %v", err)
	}

	// Must still work (EVAL fallback), NOT degrade to local fallback
	res := lim.Allow(ctx, uniqueKey("it:ns2"))
	if !res.Allowed || res.DegradedMode {
		t.Errorf("limiter must self-heal after SCRIPT FLUSH, got %+v", res)
	}
	if lim.CircuitBreakerState() != StateClosed {
		t.Errorf("breaker should stay closed, got %s", lim.CircuitBreakerState())
	}
}

// TestIntegration_GCRA verifies the GCRA script enforces the same burst
// semantics: capacity requests pass instantly, the next is denied with a
// sensible retry-after.
func TestIntegration_GCRA(t *testing.T) {
	rdb := testRedis(t)
	defer rdb.Close()

	const capacity = 20
	lim := New(rdb, StrategyGCRA, Config{
		Capacity:   capacity,
		RefillRate: 1, // 1 req/sec sustained
		Timeout:    2 * time.Second,
	})

	ctx := context.Background()
	key := uniqueKey("it:gcra")

	allowed := 0
	for i := 0; i < capacity+5; i++ {
		if res := lim.Allow(ctx, key); res.Allowed {
			allowed++
		}
	}
	// GCRA boundary rounding can differ by one from the token bucket
	if allowed < capacity-1 || allowed > capacity+1 {
		t.Errorf("GCRA burst: %d allowed, want ~%d", allowed, capacity)
	}

	res := lim.Allow(ctx, key)
	if res.Allowed {
		t.Fatal("burst exhausted — request should be denied")
	}
	if res.RetryAfter <= 0 || res.RetryAfter > 3*time.Second {
		t.Errorf("retry-after should be ~1s at 1 rps, got %v", res.RetryAfter)
	}
}

// TestIntegration_TokenBucket_Refill verifies tokens come back over time.
func TestIntegration_TokenBucket_Refill(t *testing.T) {
	rdb := testRedis(t)
	defer rdb.Close()

	lim := New(rdb, StrategyTokenBucket, Config{
		Capacity: 3, RefillRate: 50, Timeout: 2 * time.Second, // fast refill
	})

	ctx := context.Background()
	key := uniqueKey("it:refill")

	for i := 0; i < 3; i++ {
		lim.Allow(ctx, key)
	}
	if res := lim.Allow(ctx, key); res.Allowed {
		t.Fatal("bucket should be empty")
	}

	time.Sleep(100 * time.Millisecond) // 50/s × 0.1s = ~5 tokens back (capped at 3)

	if res := lim.Allow(ctx, key); !res.Allowed {
		t.Error("bucket should have refilled")
	}
}
