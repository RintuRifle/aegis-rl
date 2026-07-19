package handlers

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

// Stats holds real-time counters for the dashboard API.
type Stats struct {
	AllowedCount  atomic.Int64
	DeniedCount   atomic.Int64
	DegradedCount atomic.Int64
	StartTime     time.Time
}

// NewStats creates a new Stats instance.
func NewStats() *Stats {
	return &Stats{
		StartTime: time.Now(),
	}
}

// StatsResponse is the JSON response for /api/stats.
type StatsResponse struct {
	Allowed       int64   `json:"allowed"`
	Denied        int64   `json:"denied"`
	Degraded      int64   `json:"degraded"`
	Total         int64   `json:"total"`
	UptimeSeconds float64 `json:"uptime_seconds"`
	Timestamp     int64   `json:"timestamp"`
}

// StatsHandler returns an HTTP handler for the /api/stats endpoint.
func StatsHandler(s *Stats) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		allowed := s.AllowedCount.Load()
		denied := s.DeniedCount.Load()
		degraded := s.DegradedCount.Load()

		resp := StatsResponse{
			Allowed:       allowed,
			Denied:        denied,
			Degraded:      degraded,
			Total:         allowed + denied,
			UptimeSeconds: time.Since(s.StartTime).Seconds(),
			Timestamp:     time.Now().UnixMilli(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// HealthHandler returns 200 OK for health checks (Caddy, monitoring, etc).
func HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"aegisrl"}`))
	}
}

// DemoHandler is a simple handler that simulates a protected API endpoint.
func DemoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message":"Hello from AegisRL! This request was allowed through the rate limiter.","timestamp":` +
			`"` + time.Now().Format(time.RFC3339) + `"}`))
	}
}

// ConfigHandler returns the current rate limiter config (for dashboard display).
type TierInfo struct {
	Name       string  `json:"name"`
	Capacity   int64   `json:"capacity"`
	RefillRate float64 `json:"refill_rate"`
}

// ConfigHandler serves the active strategy and the LIVE tier table.
// tiers is a getter (not a snapshot) so hot-reloaded tiers appear immediately.
func ConfigHandler(strategy string, tiers func() map[string]TierInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"strategy": strategy,
			"tiers":    tiers(),
		})
	}
}
