package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

// TierStore holds the live tier table behind an atomic.Value so the hot path
// reads lock-free while a background watcher swaps in updated config.
// This is the "config hot-reload" piece: a Pro customer can be upgraded by
// editing the tiers file — no process restart, no dropped requests.
type TierStore struct {
	tiers atomic.Value // map[string]Tier — replaced wholesale, never mutated
}

// NewTierStore creates a store seeded with an initial tier table.
func NewTierStore(initial map[string]Tier) *TierStore {
	s := &TierStore{}
	s.Replace(initial)
	return s
}

// Replace swaps in a new tier table (copied defensively).
func (s *TierStore) Replace(m map[string]Tier) {
	cp := make(map[string]Tier, len(m))
	for k, v := range m {
		cp[k] = v
	}
	s.tiers.Store(cp)
}

// Get returns the tier for a name, lock-free.
func (s *TierStore) Get(name string) (Tier, bool) {
	m := s.tiers.Load().(map[string]Tier)
	t, ok := m[name]
	return t, ok
}

// All returns a copy of the current tier table.
func (s *TierStore) All() map[string]Tier {
	m := s.tiers.Load().(map[string]Tier)
	cp := make(map[string]Tier, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

// LoadTiersFile parses a JSON array of tiers from disk:
//
//	[{"name":"free","capacity":100,"refill_rate":10}, ...]
func LoadTiersFile(path string) (map[string]Tier, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tiers []Tier
	if err := json.Unmarshal(data, &tiers); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	m := make(map[string]Tier, len(tiers))
	for _, t := range tiers {
		if t.Name == "" || t.Capacity <= 0 || t.RefillRate <= 0 {
			return nil, fmt.Errorf("invalid tier %+v in %s", t, path)
		}
		m[t.Name] = t
	}
	return m, nil
}

// WatchFile polls the file's mtime every interval and hot-reloads the tier
// table when it changes. A malformed file is reported via onReload and the
// previous (known-good) table stays active — a bad deploy can't break limits.
//
// Polling (vs fsnotify) is deliberate: zero extra dependencies, works on every
// OS/volume type, and a few seconds of propagation delay is irrelevant for
// rate limit config.
func (s *TierStore) WatchFile(ctx context.Context, path string, interval time.Duration, onReload func(count int, err error)) {
	go func() {
		var lastMod time.Time
		if fi, err := os.Stat(path); err == nil {
			lastMod = fi.ModTime()
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fi, err := os.Stat(path)
				if err != nil {
					continue // file temporarily missing (e.g. atomic rename in progress)
				}
				if !fi.ModTime().After(lastMod) {
					continue
				}
				lastMod = fi.ModTime()
				m, err := LoadTiersFile(path)
				if err != nil {
					if onReload != nil {
						onReload(0, err)
					}
					continue // keep serving the previous known-good config
				}
				s.Replace(m)
				if onReload != nil {
					onReload(len(m), nil)
				}
			}
		}
	}()
}
