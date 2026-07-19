package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTierStore_GetAndReplace(t *testing.T) {
	s := NewTierStore(map[string]Tier{
		"free": {Name: "free", Capacity: 100, RefillRate: 10},
	})

	if tier, ok := s.Get("free"); !ok || tier.Capacity != 100 {
		t.Fatalf("expected free/100, got %+v ok=%v", tier, ok)
	}
	if _, ok := s.Get("pro"); ok {
		t.Fatal("pro should not exist yet")
	}

	s.Replace(map[string]Tier{
		"pro": {Name: "pro", Capacity: 1000, RefillRate: 100},
	})
	if _, ok := s.Get("free"); ok {
		t.Error("free should be gone after Replace")
	}
	if tier, ok := s.Get("pro"); !ok || tier.Capacity != 1000 {
		t.Errorf("expected pro/1000 after Replace, got %+v ok=%v", tier, ok)
	}
}

func TestLoadTiersFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tiers.json")
	os.WriteFile(path, []byte(`[
		{"name":"free","capacity":50,"refill_rate":5},
		{"name":"pro","capacity":500,"refill_rate":50}
	]`), 0o644)

	m, err := LoadTiersFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m) != 2 || m["pro"].Capacity != 500 {
		t.Errorf("unexpected tiers: %+v", m)
	}
}

func TestLoadTiersFile_RejectsInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tiers.json")
	os.WriteFile(path, []byte(`[{"name":"broken","capacity":0,"refill_rate":5}]`), 0o644)

	if _, err := LoadTiersFile(path); err == nil {
		t.Error("expected error for capacity=0 tier")
	}
}

func TestTierStore_WatchFile_HotReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tiers.json")
	os.WriteFile(path, []byte(`[{"name":"free","capacity":100,"refill_rate":10}]`), 0o644)

	s := NewTierStore(map[string]Tier{
		"free": {Name: "free", Capacity: 100, RefillRate: 10},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reloaded := make(chan int, 4)
	s.WatchFile(ctx, path, 10*time.Millisecond, func(count int, err error) {
		if err == nil {
			reloaded <- count
		}
	})

	// Update the file — bump mtime into the future to defeat coarse mtime granularity
	os.WriteFile(path, []byte(`[{"name":"free","capacity":999,"refill_rate":10}]`), 0o644)
	future := time.Now().Add(2 * time.Second)
	os.Chtimes(path, future, future)

	select {
	case <-reloaded:
		if tier, ok := s.Get("free"); !ok || tier.Capacity != 999 {
			t.Errorf("expected hot-reloaded free/999, got %+v ok=%v", tier, ok)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("hot reload did not fire within 3s")
	}
}

func TestTierStore_WatchFile_KeepsOldConfigOnBadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tiers.json")
	os.WriteFile(path, []byte(`[{"name":"free","capacity":100,"refill_rate":10}]`), 0o644)

	s := NewTierStore(map[string]Tier{
		"free": {Name: "free", Capacity: 100, RefillRate: 10},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	failed := make(chan error, 4)
	s.WatchFile(ctx, path, 10*time.Millisecond, func(count int, err error) {
		if err != nil {
			failed <- err
		}
	})

	// Write garbage — the previous known-good config must survive
	os.WriteFile(path, []byte(`this is not json`), 0o644)
	future := time.Now().Add(2 * time.Second)
	os.Chtimes(path, future, future)

	select {
	case <-failed:
		if tier, ok := s.Get("free"); !ok || tier.Capacity != 100 {
			t.Errorf("bad file must not clobber good config, got %+v ok=%v", tier, ok)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("watcher did not report the bad file within 3s")
	}
}
