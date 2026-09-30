package usagehistory

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestStoreAggregatesByHourAndModel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "usage-history.json")
	store := NewStore(path, 0)

	base := time.Date(2026, 9, 30, 10, 15, 0, 0, time.UTC)
	store.Add(Sample{
		Timestamp:     base,
		Provider:      "openai",
		Model:         "gpt-4o",
		UncachedInput: 1000,
		OutputTokens:  500,
		TotalTokens:   1500,
	})
	store.Add(Sample{
		Timestamp:     base.Add(20 * time.Minute), // same hour
		Provider:      "openai",
		Model:         "gpt-4o",
		UncachedInput: 2000,
		OutputTokens:  1000,
		TotalTokens:   3000,
	})

	buckets := store.Snapshot()
	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(buckets))
	}
	b := buckets[0]
	if b.Requests != 2 {
		t.Errorf("expected 2 requests, got %d", b.Requests)
	}
	if b.InputTokens != 3000 {
		t.Errorf("expected 3000 input tokens, got %d", b.InputTokens)
	}
	if b.OutputTokens != 1500 {
		t.Errorf("expected 1500 output tokens, got %d", b.OutputTokens)
	}
	if b.TotalTokens != 4500 {
		t.Errorf("expected 4500 total tokens, got %d", b.TotalTokens)
	}
	// gpt-4o: input 2.5/M, output 10/M => 3000/1e6*2.5 + 1500/1e6*10 = 0.0075 + 0.015 = 0.0225
	if b.CostUSD < 0.0224 || b.CostUSD > 0.0226 {
		t.Errorf("unexpected cost estimate: %f", b.CostUSD)
	}
}

func TestStoreSeparateHoursAndModels(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "u.json"), 0)

	base := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	store.Add(Sample{Timestamp: base, Provider: "openai", Model: "gpt-4o", TotalTokens: 10})
	store.Add(Sample{Timestamp: base.Add(time.Hour), Provider: "openai", Model: "gpt-4o", TotalTokens: 20})
	store.Add(Sample{Timestamp: base, Provider: "anthropic", Model: "claude-sonnet-4", TotalTokens: 30})

	if got := len(store.Snapshot()); got != 3 {
		t.Fatalf("expected 3 buckets, got %d", got)
	}
}

func TestStorePersistAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "usage-history.json")

	store := NewStore(path, 0)
	base := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	store.Add(Sample{Timestamp: base, Provider: "xai", Model: "grok-4", UncachedInput: 100, OutputTokens: 50, TotalTokens: 150})
	if err := store.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}

	reloaded := NewStore(path, 0)
	buckets := reloaded.Snapshot()
	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket after reload, got %d", len(buckets))
	}
	if buckets[0].TotalTokens != 150 {
		t.Errorf("expected 150 total tokens, got %d", buckets[0].TotalTokens)
	}
}

func TestStorePrunesExpiredBuckets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "usage-history.json")
	store := NewStore(path, 7) // 7-day retention

	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	store.nowFunc = fixedClock(now)

	store.Add(Sample{Timestamp: now.Add(-10 * 24 * time.Hour), Provider: "openai", Model: "gpt-4o", TotalTokens: 10})
	store.Add(Sample{Timestamp: now.Add(-1 * 24 * time.Hour), Provider: "openai", Model: "gpt-4o", TotalTokens: 20})

	if err := store.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	buckets := store.Snapshot()
	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket after prune, got %d", len(buckets))
	}
	if buckets[0].TotalTokens != 20 {
		t.Errorf("expected the recent bucket to remain, got %d", buckets[0].TotalTokens)
	}
}

func TestStoreClear(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "u.json"), 0)
	store.Add(Sample{Provider: "openai", Model: "gpt-4o", TotalTokens: 10})
	if err := store.Clear(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if got := len(store.Snapshot()); got != 0 {
		t.Errorf("expected 0 buckets after clear, got %d", got)
	}
}

func TestRateForLongestPrefixWins(t *testing.T) {
	rate, ok := rateFor("gpt-4o-mini")
	if !ok {
		t.Fatal("expected a rate for gpt-4o-mini")
	}
	if rate.InputPerM != 0.15 {
		t.Errorf("expected gpt-4o-mini input rate 0.15, got %f", rate.InputPerM)
	}

	if _, ok := rateFor("totally-unknown-model"); ok {
		t.Error("expected no rate for unknown model")
	}
}

func TestEstimateCostUsesCacheDefaults(t *testing.T) {
	rate := ModelRate{InputPerM: 3.0, OutputPerM: 15.0}
	// cacheRead defaults to 10% of input (0.3/M), cacheWrite defaults to input (3/M)
	cost := estimateCost(rate, 1_000_000, 1_000_000, 1_000_000, 1_000_000)
	// 3 + 0.3 + 3 + 15 = 21.3
	if cost < 21.29 || cost > 21.31 {
		t.Errorf("unexpected cost: %f", cost)
	}
}
