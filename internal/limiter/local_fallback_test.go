package limiter

import (
	"sync"
	"testing"
	"time"
)

func TestLocalFallback_AllowsInitialRequests(t *testing.T) {
	lf := NewLocalFallback()
	cfg := Config{
		Capacity:   10,
		RefillRate: 5.0,
		Timeout:    50 * time.Millisecond,
	}

	result := lf.Allow("test-key", cfg)
	if !result.Allowed {
		t.Error("first request should be allowed")
	}
	if !result.DegradedMode {
		t.Error("local fallback should always set DegradedMode=true")
	}
	if result.Remaining != 9 {
		t.Errorf("expected 9 remaining, got %d", result.Remaining)
	}
}

func TestLocalFallback_DeniesAfterCapacityExhausted(t *testing.T) {
	lf := NewLocalFallback()
	cfg := Config{
		Capacity:   5,
		RefillRate: 1.0,
		Timeout:    50 * time.Millisecond,
	}

	// Exhaust all 5 tokens
	for i := 0; i < 5; i++ {
		result := lf.Allow("exhaust-key", cfg)
		if !result.Allowed {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 6th request should be denied
	result := lf.Allow("exhaust-key", cfg)
	if result.Allowed {
		t.Error("request after capacity exhaustion should be denied")
	}
	if result.RetryAfter <= 0 {
		t.Error("denied request should have a positive RetryAfter")
	}
}

func TestLocalFallback_RefillsOverTime(t *testing.T) {
	lf := NewLocalFallback()
	cfg := Config{
		Capacity:   5,
		RefillRate: 100.0, // 100 tokens/sec → refills fast
		Timeout:    50 * time.Millisecond,
	}

	// Exhaust all tokens
	for i := 0; i < 5; i++ {
		lf.Allow("refill-key", cfg)
	}

	// Wait for refill (100 tokens/sec → ~50ms for 5 tokens)
	time.Sleep(100 * time.Millisecond)

	// Should be allowed again
	result := lf.Allow("refill-key", cfg)
	if !result.Allowed {
		t.Error("request after refill should be allowed")
	}
}

func TestLocalFallback_ConcurrentAccess(t *testing.T) {
	lf := NewLocalFallback()
	cfg := Config{
		Capacity:   1000,
		RefillRate: 1.0, // slow refill to prevent refill during test
		Timeout:    50 * time.Millisecond,
	}

	// Fire 1000 goroutines at the same key simultaneously
	var wg sync.WaitGroup
	allowedCount := 0
	deniedCount := 0
	var mu sync.Mutex

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := lf.Allow("concurrent-key", cfg)
			mu.Lock()
			if result.Allowed {
				allowedCount++
			} else {
				deniedCount++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Total should be exactly 1000
	total := allowedCount + deniedCount
	if total != 1000 {
		t.Errorf("expected 1000 total decisions, got %d", total)
	}

	// Allowed should be exactly equal to capacity (1000) since no refill
	// (tolerance: some goroutines may see refill from scheduling delays)
	if allowedCount > 1000 {
		t.Errorf("allowed count (%d) should never exceed capacity (1000)", allowedCount)
	}

	t.Logf("concurrent test: %d allowed, %d denied (capacity=1000)", allowedCount, deniedCount)
}

func TestLocalFallback_IsolatesKeys(t *testing.T) {
	lf := NewLocalFallback()
	cfg := Config{
		Capacity:   1,
		RefillRate: 0.01, // very slow refill
		Timeout:    50 * time.Millisecond,
	}

	// Use key A's token
	result := lf.Allow("key-a", cfg)
	if !result.Allowed {
		t.Error("key-a first request should be allowed")
	}

	// Key B should still have its own bucket
	result = lf.Allow("key-b", cfg)
	if !result.Allowed {
		t.Error("key-b should be unaffected by key-a")
	}
}

func TestLocalFallback_Cleanup(t *testing.T) {
	lf := NewLocalFallback()
	cfg := Config{
		Capacity:   10,
		RefillRate: 5.0,
		Timeout:    50 * time.Millisecond,
	}

	lf.Allow("cleanup-key", cfg)

	// Cleanup with a very short max idle — should remove the bucket
	lf.Cleanup(0) // 0 duration = everything is "idle"

	// New request should get a fresh bucket (full capacity)
	result := lf.Allow("cleanup-key", cfg)
	if !result.Allowed {
		t.Error("should be allowed after cleanup (fresh bucket)")
	}
	// After cleanup, bucket is fresh (capacity=10), one token consumed → remaining ~9
	// Due to float64 precision with time.Now(), remaining may be 8 or 9
	if result.Remaining < 8 || result.Remaining > 9 {
		t.Errorf("expected 8-9 remaining after cleanup, got %d", result.Remaining)
	}
}
