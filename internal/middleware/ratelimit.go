package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/RintuRifle/aegis-rl/internal/config"
	"github.com/RintuRifle/aegis-rl/internal/limiter"
	"github.com/RintuRifle/aegis-rl/internal/metrics"
	"go.uber.org/zap"
)

// Options wires multi-tier + per-endpoint resolution into the middleware.
type Options struct {
	Tiers       *config.TierStore     // live (hot-reloadable) tier table
	APIKeys     map[string]string     // api key → tier name
	DefaultTier string                // tier for unknown keys / anonymous IPs
	Rules       []config.EndpointRule // per-path-prefix overrides
	TrustProxy  bool                  // honor XFF/X-Real-IP only behind a proxy
}

// RateLimit returns an HTTP middleware that enforces rate limiting
// using the provided Limiter. Sets standard rate-limit headers on every response.
//
// Hot path notes: header values are built with strconv (not fmt.Sprintf) to
// avoid interface boxing allocations on every request.
func RateLimit(l *limiter.Limiter, m *metrics.Metrics, log *zap.Logger, opts Options) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity := extractIdentity(r, opts.TrustProxy)
			cfg, scope, tierName := resolveConfig(l, r.URL.Path, identity, opts)
			key := "rl:" + scope + identity

			// Measure decision latency
			start := time.Now()
			result := l.AllowWithConfig(r.Context(), key, cfg)
			elapsed := time.Since(start)

			// Record metrics
			m.DecisionLatency.Observe(elapsed.Seconds())
			m.CircuitState.Set(float64(l.CircuitBreakerState()))

			if result.DegradedMode {
				m.DegradedTotal.Inc()
			}

			// Set rate limit headers on EVERY response (allowed or denied)
			h := w.Header()
			h.Set("X-RateLimit-Limit", strconv.FormatInt(cfg.Capacity, 10))
			h.Set("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))
			// Reset = when the bucket is full again, given current fill level
			if cfg.RefillRate > 0 {
				refill := time.Duration(float64(cfg.Capacity-result.Remaining) / cfg.RefillRate * float64(time.Second))
				h.Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(refill).Unix(), 10))
			}
			if tierName != "" {
				h.Set("X-RateLimit-Tier", tierName)
			}
			if result.DegradedMode {
				h.Set("X-RateLimit-Mode", "degraded")
			}

			if !result.Allowed {
				m.RequestsTotal.WithLabelValues("denied").Inc()

				log.Info("request denied",
					zap.String("identity", identity),
					zap.String("tier", tierName),
					zap.String("scope", scope),
					zap.Int64("remaining", result.Remaining),
					zap.Duration("retry_after", result.RetryAfter),
					zap.Duration("decision_latency", elapsed),
					zap.Bool("degraded", result.DegradedMode),
				)

				// Retry-After is defined in whole seconds — round UP so clients
				// never retry a moment too early and get denied again.
				retrySec := int64((result.RetryAfter + time.Second - time.Nanosecond) / time.Second)
				h.Set("Retry-After", strconv.FormatInt(retrySec, 10))
				h.Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)

				body := make([]byte, 0, 64)
				body = append(body, `{"error":"rate_limit_exceeded","retry_after_ms":`...)
				body = strconv.AppendInt(body, result.RetryAfter.Milliseconds(), 10)
				body = append(body, '}')
				w.Write(body)
				return
			}

			m.RequestsTotal.WithLabelValues("allowed").Inc()

			// Log allowed requests at debug level only (too noisy for info)
			log.Debug("request allowed",
				zap.String("identity", identity),
				zap.Int64("remaining", result.Remaining),
				zap.Duration("decision_latency", elapsed),
			)

			next.ServeHTTP(w, r)
		})
	}
}

// resolveConfig picks the limiter config for this request.
// Priority: per-endpoint rule (longest prefix wins) → API key tier →
// default tier → limiter default config.
// Endpoint-scoped buckets get their own key namespace ("ep:<prefix>:") so a
// client's /api/search budget is independent of their main tier bucket.
func resolveConfig(l *limiter.Limiter, path, identity string, opts Options) (limiter.Config, string, string) {
	base := l.GetConfig()

	var best *config.EndpointRule
	for i := range opts.Rules {
		rule := &opts.Rules[i]
		if strings.HasPrefix(path, rule.Prefix) {
			if best == nil || len(rule.Prefix) > len(best.Prefix) {
				best = rule
			}
		}
	}
	if best != nil {
		return limiter.Config{
			Capacity:   best.Capacity,
			RefillRate: best.RefillRate,
			Timeout:    base.Timeout,
		}, "ep:" + best.Prefix + ":", ""
	}

	tierName := ""
	if strings.HasPrefix(identity, "key:") && opts.APIKeys != nil {
		tierName = opts.APIKeys[strings.TrimPrefix(identity, "key:")]
	}
	if tierName == "" {
		tierName = opts.DefaultTier
	}
	if opts.Tiers != nil && tierName != "" {
		if t, ok := opts.Tiers.Get(tierName); ok {
			return limiter.Config{
				Capacity:   t.Capacity,
				RefillRate: t.RefillRate,
				Timeout:    base.Timeout,
			}, "", tierName
		}
	}
	return base, "", ""
}
