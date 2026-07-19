package middleware

import (
	"testing"
	"time"

	"github.com/RintuRifle/aegis-rl/internal/config"
	"github.com/RintuRifle/aegis-rl/internal/limiter"
)

func testOptions() Options {
	store := config.NewTierStore(map[string]config.Tier{
		"free": {Name: "free", Capacity: 100, RefillRate: 10},
		"pro":  {Name: "pro", Capacity: 1000, RefillRate: 100},
	})
	return Options{
		Tiers:       store,
		APIKeys:     map[string]string{"pro-key": "pro"},
		DefaultTier: "free",
		Rules: []config.EndpointRule{
			{Prefix: "/api/search", Capacity: 10, RefillRate: 1},
			{Prefix: "/api/search/deep", Capacity: 2, RefillRate: 0.1},
		},
	}
}

func testLimiter() *limiter.Limiter {
	return limiter.New(nil, limiter.StrategyTokenBucket, limiter.Config{
		Capacity: 50, RefillRate: 5, Timeout: 50 * time.Millisecond,
	})
}

func TestResolveConfig_APIKeyTier(t *testing.T) {
	cfg, scope, tier := resolveConfig(testLimiter(), "/api/test", "key:pro-key", testOptions())
	if tier != "pro" || cfg.Capacity != 1000 || scope != "" {
		t.Errorf("expected pro tier (cap 1000), got tier=%q cap=%d scope=%q", tier, cfg.Capacity, scope)
	}
}

func TestResolveConfig_UnknownKeyGetsDefaultTier(t *testing.T) {
	cfg, _, tier := resolveConfig(testLimiter(), "/api/test", "key:mystery", testOptions())
	if tier != "free" || cfg.Capacity != 100 {
		t.Errorf("unknown key should get default tier free/100, got %q/%d", tier, cfg.Capacity)
	}
}

func TestResolveConfig_AnonymousIPGetsDefaultTier(t *testing.T) {
	cfg, _, tier := resolveConfig(testLimiter(), "/api/test", "ip:1.2.3.4", testOptions())
	if tier != "free" || cfg.Capacity != 100 {
		t.Errorf("anonymous IP should get default tier free/100, got %q/%d", tier, cfg.Capacity)
	}
}

func TestResolveConfig_EndpointRuleWins(t *testing.T) {
	cfg, scope, _ := resolveConfig(testLimiter(), "/api/search", "key:pro-key", testOptions())
	if cfg.Capacity != 10 || scope != "ep:/api/search:" {
		t.Errorf("endpoint rule should override tier, got cap=%d scope=%q", cfg.Capacity, scope)
	}
}

func TestResolveConfig_LongestPrefixWins(t *testing.T) {
	cfg, scope, _ := resolveConfig(testLimiter(), "/api/search/deep/x", "ip:1.2.3.4", testOptions())
	if cfg.Capacity != 2 || scope != "ep:/api/search/deep:" {
		t.Errorf("longest matching prefix should win, got cap=%d scope=%q", cfg.Capacity, scope)
	}
}

func TestResolveConfig_NoTierStoreFallsBackToBase(t *testing.T) {
	cfg, _, tier := resolveConfig(testLimiter(), "/api/test", "ip:1.2.3.4", Options{})
	if cfg.Capacity != 50 || tier != "" {
		t.Errorf("with no tier store, base limiter config should apply, got cap=%d tier=%q", cfg.Capacity, tier)
	}
}
