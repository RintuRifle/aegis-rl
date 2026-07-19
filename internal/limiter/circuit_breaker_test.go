package limiter

import (
	"testing"
	"time"
)

func TestCircuitBreaker_StartsInClosedState(t *testing.T) {
	cb := NewCircuitBreaker(3, 5*time.Second)
	if cb.State() != StateClosed {
		t.Errorf("expected StateClosed, got %s", cb.State())
	}
	if cb.Open() {
		t.Error("circuit breaker should not be open initially")
	}
}

func TestCircuitBreaker_TripsAfterMaxFailures(t *testing.T) {
	cb := NewCircuitBreaker(3, 5*time.Second)

	// Record 3 failures (maxFails = 3)
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.Open() {
		t.Error("should not trip after 2 failures (max is 3)")
	}

	cb.RecordFailure() // 3rd failure — should trip
	if !cb.Open() {
		t.Error("circuit breaker should be open after 3 consecutive failures")
	}
	if cb.State() != StateOpen {
		t.Errorf("expected StateOpen, got %s", cb.State())
	}
}

func TestCircuitBreaker_SuccessResets(t *testing.T) {
	cb := NewCircuitBreaker(3, 5*time.Second)

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess() // resets the counter

	cb.RecordFailure()
	cb.RecordFailure()
	// Only 2 failures since last success — should NOT trip
	if cb.Open() {
		t.Error("circuit breaker should not be open after success reset")
	}
}

func TestCircuitBreaker_TransitionsToHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond) // very short cooldown for testing

	cb.RecordFailure()
	cb.RecordFailure() // trips open

	if !cb.Open() {
		t.Error("should be open")
	}

	// Wait for cooldown to expire
	time.Sleep(100 * time.Millisecond)

	// Should transition to half-open and allow one probe
	if cb.Open() {
		t.Error("should transition to half-open after cooldown")
	}
	if cb.State() != StateHalfOpen {
		t.Errorf("expected StateHalfOpen, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenRecovery(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond)

	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(100 * time.Millisecond)
	cb.Open() // transitions to half-open

	cb.RecordSuccess() // probe succeeded — close the breaker
	if cb.State() != StateClosed {
		t.Errorf("expected StateClosed after successful probe, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenFailure(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond)

	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(100 * time.Millisecond)
	cb.Open() // transitions to half-open

	cb.RecordFailure() // probe failed — should trip open again
	if cb.State() != StateOpen {
		t.Errorf("expected StateOpen after failed probe, got %s", cb.State())
	}
}

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state CircuitState
		want  string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("CircuitState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestCircuitBreaker_SingleProbeInHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond)

	cb.RecordFailure()
	cb.RecordFailure() // trips open
	time.Sleep(100 * time.Millisecond)

	// First caller after cooldown becomes the probe → goes to Redis
	if cb.Open() {
		t.Fatal("first caller after cooldown should be allowed through as the probe")
	}
	// While the probe is in flight, everyone else must use the fallback —
	// otherwise a busy server floods a still-down Redis the moment cooldown ends
	if !cb.Open() {
		t.Error("second caller should be routed to fallback while probe is in flight")
	}
	if !cb.Open() {
		t.Error("third caller should also be routed to fallback")
	}

	// Probe succeeds → breaker closes, traffic flows to Redis again
	cb.RecordSuccess()
	if cb.State() != StateClosed {
		t.Errorf("expected StateClosed after successful probe, got %s", cb.State())
	}
	if cb.Open() {
		t.Error("breaker should be closed after successful probe")
	}
}

func TestCircuitBreaker_FailedProbeReopensAndAllowsNextProbe(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond)

	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(100 * time.Millisecond)

	cb.Open()          // become the probe
	cb.RecordFailure() // probe failed → back to open

	if cb.State() != StateOpen {
		t.Fatalf("expected StateOpen after failed probe, got %s", cb.State())
	}
	// Next cooldown expiry should elect a fresh probe (probing flag must reset)
	time.Sleep(100 * time.Millisecond)
	if cb.Open() {
		t.Error("after cooldown, a new probe should be allowed through")
	}
}
