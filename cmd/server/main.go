package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof" // pprof for sub-millisecond latency profiling
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RintuRifle/aegis-rl/internal/config"
	"github.com/RintuRifle/aegis-rl/internal/handlers"
	"github.com/RintuRifle/aegis-rl/internal/limiter"
	"github.com/RintuRifle/aegis-rl/internal/logging"
	"github.com/RintuRifle/aegis-rl/internal/metrics"
	"github.com/RintuRifle/aegis-rl/internal/middleware"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	// ── Load config ──────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// ── Structured logger ────────────────────────────────────
	log := logging.New(cfg.LogLevel)
	defer log.Sync()

	log.Info("starting AegisRL rate limiter",
		zap.String("listen", cfg.ListenAddr),
		zap.String("metrics", cfg.MetricsAddr),
		zap.String("redis", cfg.RedisAddr),
		zap.String("strategy", cfg.Strategy),
		zap.Bool("trust_proxy", cfg.TrustProxy),
		zap.Int64("default_capacity", cfg.DefaultCapacity),
		zap.Float64("default_refill_rate", cfg.DefaultRefillRate),
	)

	// ── Redis client ─────────────────────────────────────────
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		Password:     cfg.RedisPassword,
		DB:           cfg.RedisDB,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  cfg.RedisTimeout,
		WriteTimeout: cfg.RedisTimeout,
		PoolSize:     50,
		MinIdleConns: 10,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test Redis connection (non-fatal — we have local fallback)
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Warn("redis connection failed — starting in degraded mode",
			zap.Error(err),
		)
	} else {
		log.Info("redis connected successfully")
	}

	// ── Prometheus metrics ───────────────────────────────────
	m := metrics.New()

	// ── Create limiter ───────────────────────────────────────
	limiterCfg := limiter.Config{
		Capacity:   cfg.DefaultCapacity,
		RefillRate: cfg.DefaultRefillRate,
		Timeout:    cfg.RedisTimeout,
	}
	lim := limiter.New(rdb, limiter.Strategy(cfg.Strategy), limiterCfg)
	lim.OnRedisLatency(func(d time.Duration) {
		m.RedisLatency.Observe(d.Seconds())
	})

	// Warm the Redis script cache (optional — Run() self-heals on NOSCRIPT)
	if sha, err := lim.Preload(ctx); err != nil {
		log.Warn("lua script preload failed — will lazy-load on first request",
			zap.Error(err),
		)
	} else {
		log.Info("lua script preloaded", zap.String("sha", sha))
	}

	// Start background cleanup for local fallback buckets
	lim.StartCleanup(ctx, 30*time.Second)

	// ── Tier store (hot-reloadable) ──────────────────────────
	tierStore := config.NewTierStore(cfg.Tiers)
	if cfg.TiersFile != "" {
		// Seed from file if it already exists, then watch for changes
		if m0, err := config.LoadTiersFile(cfg.TiersFile); err == nil {
			tierStore.Replace(m0)
			log.Info("tiers loaded from file",
				zap.String("path", cfg.TiersFile), zap.Int("count", len(m0)))
		}
		tierStore.WatchFile(ctx, cfg.TiersFile, 5*time.Second, func(count int, err error) {
			if err != nil {
				log.Error("tier hot-reload failed — keeping previous config", zap.Error(err))
				return
			}
			log.Info("tiers hot-reloaded", zap.Int("count", count))
		})
	}

	// ── Stats for dashboard ──────────────────────────────────
	stats := handlers.NewStats()

	// ── Build HTTP mux ───────────────────────────────────────
	mux := http.NewServeMux()

	// Health check (unauthenticated, no rate limit)
	mux.HandleFunc("/healthz", handlers.HealthHandler())

	// Stats API for dashboard
	mux.HandleFunc("/api/stats", handlers.StatsHandler(stats))

	// Config API for dashboard — reads the LIVE tier store, so hot-reloaded
	// tiers show up without a restart
	mux.HandleFunc("/api/config", handlers.ConfigHandler(cfg.Strategy, func() map[string]handlers.TierInfo {
		all := tierStore.All()
		out := make(map[string]handlers.TierInfo, len(all))
		for name, t := range all {
			out[name] = handlers.TierInfo{Name: t.Name, Capacity: t.Capacity, RefillRate: t.RefillRate}
		}
		return out
	}))

	// Demo API endpoint (protected by rate limiter)
	mux.HandleFunc("/api/test", handlers.DemoHandler())

	// Catch-all for any other API routes
	mux.HandleFunc("/api/", handlers.DemoHandler())

	// ── Middleware chain ──────────────────────────────────────
	// Order: CORS → RateLimit → Handler
	var handler http.Handler = mux
	handler = middleware.RateLimit(lim, m, log, middleware.Options{
		Tiers:       tierStore,
		APIKeys:     cfg.APIKeys,
		DefaultTier: cfg.DefaultTier,
		Rules:       cfg.EndpointRules,
		TrustProxy:  cfg.TrustProxy,
	})(handler)
	handler = middleware.CORS(cfg.DashboardOrigin)(handler)

	// Wrap with stats counting middleware
	handler = statsMiddleware(stats, handler)

	// ── Start metrics server (separate port) ─────────────────
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", metrics.Handler())
	metricsMux.HandleFunc("/debug/pprof/", http.DefaultServeMux.ServeHTTP)

	metricsServer := &http.Server{
		Addr:    cfg.MetricsAddr,
		Handler: metricsMux,
	}

	go func() {
		log.Info("metrics server starting", zap.String("addr", cfg.MetricsAddr))
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("metrics server failed", zap.Error(err))
		}
	}()

	// ── Start main HTTP server ───────────────────────────────
	server := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Info("AegisRL edge bouncer starting", zap.String("addr", cfg.ListenAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server failed", zap.Error(err))
		}
	}()

	// ── Graceful shutdown ────────────────────────────────────
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Info("received shutdown signal", zap.String("signal", sig.String()))

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	// Shutdown both servers
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("server shutdown error", zap.Error(err))
	}
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		log.Error("metrics server shutdown error", zap.Error(err))
	}

	// Close Redis connection
	rdb.Close()
	cancel()

	log.Info("AegisRL shut down gracefully")
}

// statsMiddleware increments stats counters based on response status codes.
func statsMiddleware(s *handlers.Stats, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wrap response writer to capture status code
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		// Update stats based on outcome
		if sw.status == http.StatusTooManyRequests {
			s.DeniedCount.Add(1)
		} else if sw.status < 400 {
			s.AllowedCount.Add(1)
		}

		// Check for degraded mode header
		if w.Header().Get("X-RateLimit-Mode") == "degraded" {
			s.DegradedCount.Add(1)
		}
	})
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}
