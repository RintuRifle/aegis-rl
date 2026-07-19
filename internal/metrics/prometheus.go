package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for AegisRL.
type Metrics struct {
	RequestsTotal    *prometheus.CounterVec
	DegradedTotal    prometheus.Counter
	DecisionLatency  prometheus.Histogram
	RedisLatency     prometheus.Histogram
	CircuitState     prometheus.Gauge
}

// New registers and returns all AegisRL Prometheus metrics.
func New() *Metrics {
	m := &Metrics{
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "aegisrl",
				Name:      "requests_total",
				Help:      "Total number of rate-limited requests by status.",
			},
			[]string{"status"}, // "allowed" or "denied"
		),
		DegradedTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "aegisrl",
				Name:      "requests_degraded_total",
				Help:      "Total number of requests served by local fallback (degraded mode).",
			},
		),
		DecisionLatency: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: "aegisrl",
				Name:      "decision_latency_seconds",
				Help:      "Latency of the rate-limit decision (Redis or local), in seconds.",
				Buckets:   []float64{0.0001, 0.00025, 0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1},
			},
		),
		RedisLatency: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: "aegisrl",
				Name:      "redis_latency_seconds",
				Help:      "Latency of Redis EVALSHA calls, in seconds.",
				Buckets:   []float64{0.0001, 0.00025, 0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1},
			},
		),
		CircuitState: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "aegisrl",
				Name:      "circuit_breaker_state",
				Help:      "Current circuit breaker state: 0=closed, 1=open, 2=half-open.",
			},
		),
	}

	prometheus.MustRegister(
		m.RequestsTotal,
		m.DegradedTotal,
		m.DecisionLatency,
		m.RedisLatency,
		m.CircuitState,
	)

	return m
}

// Handler returns the Prometheus metrics HTTP handler.
func Handler() http.Handler {
	return promhttp.Handler()
}
