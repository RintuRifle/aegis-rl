package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Tier represents a rate limiting tier with its own capacity and refill rate.
type Tier struct {
	Name       string  `json:"name"`
	Capacity   int64   `json:"capacity"`     // max burst
	RefillRate float64 `json:"refill_rate"`  // tokens/sec
}

// EndpointRule applies a stricter (or looser) limit to a URL path prefix,
// e.g. give /api/search its own budget separate from the client's tier bucket.
type EndpointRule struct {
	Prefix     string  `json:"prefix"`
	Capacity   int64   `json:"capacity"`
	RefillRate float64 `json:"refill_rate"`
}

// Config holds the application-wide configuration, loaded from environment variables.
type Config struct {
	// Server
	ListenAddr  string `json:"listen_addr"`
	MetricsAddr string `json:"metrics_addr"`

	// Redis
	RedisAddr     string `json:"redis_addr"`
	RedisPassword string `json:"redis_password"`
	RedisDB       int    `json:"redis_db"`

	// Rate Limiting defaults
	DefaultCapacity   int64         `json:"default_capacity"`
	DefaultRefillRate float64       `json:"default_refill_rate"`
	RedisTimeout      time.Duration `json:"redis_timeout"`

	// Strategy: "token_bucket" (default) or "gcra"
	Strategy string `json:"strategy"`

	// Tiers
	Tiers       map[string]Tier `json:"tiers"`
	DefaultTier string          `json:"default_tier"`
	// TiersFile: optional path to a JSON file of tiers, hot-reloaded on change
	// (mtime polled — no restart needed to change a customer's limits).
	TiersFile string `json:"tiers_file"`

	// APIKeys maps an API key to a tier name, e.g. {"abc123":"pro"}.
	APIKeys map[string]string `json:"api_keys"`

	// EndpointRules give specific path prefixes their own limits.
	EndpointRules []EndpointRule `json:"endpoint_rules"`

	// TrustProxy: only honor X-Forwarded-For / X-Real-IP when the service
	// actually sits behind a trusted reverse proxy (Caddy). When false,
	// those client-controlled headers are ignored — otherwise a direct
	// caller could rotate identities by faking XFF.
	TrustProxy bool `json:"trust_proxy"`

	// Logging
	LogLevel string `json:"log_level"`

	// Dashboard CORS
	DashboardOrigin string `json:"dashboard_origin"`
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		ListenAddr:        envOrDefault("LISTEN_ADDR", ":8081"),
		MetricsAddr:       envOrDefault("METRICS_ADDR", ":9100"),
		RedisAddr:         envOrDefault("REDIS_ADDR", "localhost:6379"),
		RedisPassword:     os.Getenv("REDIS_PASSWORD"),
		RedisDB:           envIntOrDefault("REDIS_DB", 0),
		DefaultCapacity:   int64(envIntOrDefault("CAPACITY", 100)),
		DefaultRefillRate: envFloatOrDefault("REFILL_RATE", 10.0),
		RedisTimeout:      time.Duration(envIntOrDefault("TIMEOUT_MS", 50)) * time.Millisecond,
		Strategy:          envOrDefault("STRATEGY", "token_bucket"),
		DefaultTier:       envOrDefault("DEFAULT_TIER", "free"),
		TiersFile:         os.Getenv("TIERS_FILE"),
		TrustProxy:        envBoolOrDefault("TRUST_PROXY", false),
		LogLevel:          envOrDefault("LOG_LEVEL", "info"),
		DashboardOrigin:   envOrDefault("DASHBOARD_ORIGIN", "http://localhost:3000"),
	}

	if cfg.Strategy != "token_bucket" && cfg.Strategy != "gcra" {
		return nil, fmt.Errorf("invalid STRATEGY %q (want token_bucket or gcra)", cfg.Strategy)
	}
	if cfg.DefaultCapacity <= 0 || cfg.DefaultRefillRate <= 0 {
		return nil, fmt.Errorf("CAPACITY and REFILL_RATE must be > 0 (got %d, %g)",
			cfg.DefaultCapacity, cfg.DefaultRefillRate)
	}

	// Load tiers from TIERS env var (JSON) or use defaults
	tiersJSON := os.Getenv("TIERS")
	if tiersJSON != "" {
		var tiers []Tier
		if err := json.Unmarshal([]byte(tiersJSON), &tiers); err != nil {
			return nil, fmt.Errorf("failed to parse TIERS: %w", err)
		}
		cfg.Tiers = make(map[string]Tier, len(tiers))
		for _, t := range tiers {
			if t.Name == "" || t.Capacity <= 0 || t.RefillRate <= 0 {
				return nil, fmt.Errorf("invalid tier %+v: name required, capacity/refill_rate must be > 0", t)
			}
			cfg.Tiers[t.Name] = t
		}
	} else {
		// Default tiers
		cfg.Tiers = map[string]Tier{
			"free":       {Name: "free", Capacity: 100, RefillRate: 10},
			"pro":        {Name: "pro", Capacity: 1000, RefillRate: 100},
			"enterprise": {Name: "enterprise", Capacity: 10000, RefillRate: 1000},
		}
	}

	// API key → tier mapping, e.g. API_KEYS='{"abc123":"pro","xyz789":"enterprise"}'
	if keysJSON := os.Getenv("API_KEYS"); keysJSON != "" {
		if err := json.Unmarshal([]byte(keysJSON), &cfg.APIKeys); err != nil {
			return nil, fmt.Errorf("failed to parse API_KEYS: %w", err)
		}
	}

	// Per-endpoint rules, e.g.
	// ENDPOINT_RULES='[{"prefix":"/api/search","capacity":10,"refill_rate":1}]'
	if rulesJSON := os.Getenv("ENDPOINT_RULES"); rulesJSON != "" {
		if err := json.Unmarshal([]byte(rulesJSON), &cfg.EndpointRules); err != nil {
			return nil, fmt.Errorf("failed to parse ENDPOINT_RULES: %w", err)
		}
		for _, r := range cfg.EndpointRules {
			if r.Prefix == "" || r.Capacity <= 0 || r.RefillRate <= 0 {
				return nil, fmt.Errorf("invalid endpoint rule %+v: prefix required, capacity/refill_rate must be > 0", r)
			}
		}
	}

	return cfg, nil
}

// GetTier returns the tier config for a given tier name, falling back to "free".
func (c *Config) GetTier(name string) Tier {
	if t, ok := c.Tiers[name]; ok {
		return t
	}
	return c.Tiers["free"]
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIntOrDefault(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envFloatOrDefault(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

func envBoolOrDefault(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
