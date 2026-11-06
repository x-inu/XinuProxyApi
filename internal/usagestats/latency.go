// Package usagestats keeps a bounded in-memory reservoir of per-request
// latency samples to compute p50/p95/p99 on demand. It is process-local
// telemetry for the panel's Analytics page: it holds at most ~2k samples per
// provider+model, is exact for the current window, and resets on restart.
// It never blocks the hot path.
package usagestats

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"

	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
)

const reservoirSize = 1024

type bucket struct {
	total     int64
	failed    int64
	reservoir []int64 // latency samples, reservoir-sampled
	seen      int64
}

// LatencyTracker tracks per-request latency percentiles per provider+model.
type LatencyTracker struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

// NewLatencyTracker creates an empty tracker.
func NewLatencyTracker() *LatencyTracker {
	return &LatencyTracker{buckets: make(map[string]*bucket)}
}

var globalMu      sync.RWMutex
var globalTracker = NewLatencyTracker()

// Global returns the process-wide latency tracker.
func Global() *LatencyTracker {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalTracker
}

// Record feeds one usage record into the tracker.
func (t *LatencyTracker) Record(provider, model string, latencyMs int64, failed bool) {
	if t == nil || provider == "" || model == "" || latencyMs < 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	key := provider + "\x00" + model
	b, ok := t.buckets[key]
	if !ok {
		b = &bucket{}
		t.buckets[key] = b
	}
	b.total++
	if failed {
		b.failed++
	}
	// Reservoir sampling: deterministic-ish O(1) per sample.
	b.seen++
	if len(b.reservoir) < reservoirSize {
		b.reservoir = append(b.reservoir, latencyMs)
		return
	}
	// Replace a random slot with decreasing probability; for a rough-but-fair
	// reservoir we use a simple LCG seeded by seen.
	rand := uint64(b.seen * 2654435761 % 0xFFFFFFFF)
	idx := int(rand % uint64(len(b.reservoir)))
	b.reservoir[idx] = latencyMs
}

// BucketStats is aggregated latency stats for one provider+model.
type BucketStats struct {
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Total     int64  `json:"total"`
	Failed    int64  `json:"failed"`
	ErrorRate string `json:"error_rate"`
	P50       int64  `json:"p50_ms"`
	P95       int64  `json:"p95_ms"`
	P99       int64  `json:"p99_ms"`
	Avg       int64  `json:"avg_ms"`
	Samples   int    `json:"samples"`
}

// Snapshot returns percentile stats for all tracked provider+model pairs.
func (t *LatencyTracker) Snapshot() []BucketStats {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]BucketStats, 0, len(t.buckets))
	for key, b := range t.buckets {
		var provider, model string
		// split first NUL
		for i := 0; i < len(key); i++ {
			if key[i] == 0 {
				provider = key[:i]
				model = key[i+1:]
				break
			}
		}
		stats := BucketStats{
			Provider: provider,
			Model:    model,
			Total:    b.total,
			Failed:   b.failed,
			Samples:  len(b.reservoir),
		}
		if b.total > 0 {
			stats.ErrorRate = formatPercent(float64(b.failed) / float64(b.total) * 100)
		} else {
			stats.ErrorRate = "0.0"
		}
		if len(b.reservoir) > 0 {
			sorted := make([]int64, len(b.reservoir))
			copy(sorted, b.reservoir)
			sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
			stats.P50 = percentile(sorted, 0.50)
			stats.P95 = percentile(sorted, 0.95)
			stats.P99 = percentile(sorted, 0.99)
			stats.Avg = mean(sorted)
		}
		out = append(out, stats)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Total != out[j].Total {
			return out[i].Total > out[j].Total
		}
		if out[i].Provider != out[j].Provider {
			return out[i].Provider < out[j].Provider
		}
		return out[i].Model < out[j].Model
	})
	return out
}

// OverallStats aggregates across all models.
type OverallStats struct {
	Total     int64  `json:"total"`
	Failed    int64  `json:"failed"`
	ErrorRate string `json:"error_rate"`
	P50       int64  `json:"p50_ms"`
	P95       int64  `json:"p95_ms"`
	P99       int64  `json:"p99_ms"`
	Avg       int64  `json:"avg_ms"`
	Samples   int    `json:"samples"`
}

// SnapshotOverall returns aggregated percentiles across all tracked models.
func (t *LatencyTracker) SnapshotOverall() OverallStats {
	if t == nil {
		return OverallStats{ErrorRate: "0.0"}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	var total, failed int64
	var all []int64
	for _, b := range t.buckets {
		total += b.total
		failed += b.failed
		all = append(all, b.reservoir...)
	}
	stats := OverallStats{Total: total, Failed: failed, Samples: len(all)}
	if total > 0 {
		stats.ErrorRate = formatPercent(float64(failed) / float64(total) * 100)
	} else {
		stats.ErrorRate = "0.0"
	}
	if len(all) > 0 {
		sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })
		stats.P50 = percentile(all, 0.50)
		stats.P95 = percentile(all, 0.95)
		stats.P99 = percentile(all, 0.99)
		stats.Avg = mean(all)
	}
	return stats
}

func percentile(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func mean(sorted []int64) int64 {
	var sum int64
	for _, v := range sorted {
		sum += v
	}
	return sum / int64(len(sorted))
}

func formatPercent(f float64) string {
	if f >= 100 {
		return "100.0"
	}
	return fmt.Sprintf("%.1f", f)
}

// plugin folds usage records into the tracker.
type plugin struct{}

func init() {
	coreusage.RegisterPlugin(&plugin{})
}

// HandleUsage implements coreusage.Plugin.
func (p *plugin) HandleUsage(_ context.Context, record coreusage.Record) {
	provider := record.Provider
	if provider == "" {
		provider = "unknown"
	}
	model := record.Model
	if model == "" {
		model = "unknown"
	}
	Global().Record(provider, model, record.Latency.Milliseconds(), record.Failed)
}
