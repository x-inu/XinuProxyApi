// Package usagehistory provides a lightweight, persistent rollup of token
// usage and estimated cost. It aggregates per-request usage records into
// hourly buckets keyed by provider and model, keeping memory and storage use
// minimal. Buckets are flushed to a single JSON file with atomic writes,
// following the same durability pattern as the cooldown state store.
package usagehistory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// schemaVersion identifies the on-disk rollup format.
const schemaVersion = 1

// defaultRetentionDays bounds how much history is kept on disk.
const defaultRetentionDays = 90

// bucketKey identifies an hourly aggregation slot.
type bucketKey struct {
	Hour     int64 // Unix hour (UTC), i.e. unix seconds truncated to the hour.
	Provider string
	Model    string
}

// Bucket holds aggregated counters for a single hour/provider/model slot.
type Bucket struct {
	Hour         int64   `json:"hour"`
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	Requests     int64   `json:"requests"`
	Failed       int64   `json:"failed"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CachedTokens int64   `json:"cached_tokens"`
	TotalTokens  int64   `json:"total_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

// snapshot is the on-disk representation of the rollup.
type snapshot struct {
	SchemaVersion int       `json:"schema_version"`
	UpdatedAt     time.Time `json:"updated_at"`
	Buckets       []Bucket  `json:"buckets"`
}

// Sample is a single usage observation fed into the store.
type Sample struct {
	Timestamp     time.Time
	Provider      string
	Model         string
	Failed        bool
	UncachedInput int64
	CacheRead     int64
	CacheWrite    int64
	CachedTokens  int64
	OutputTokens  int64
	TotalTokens   int64
}

// Store accumulates usage samples into hourly buckets and persists them.
type Store struct {
	mu            sync.Mutex
	path          string
	buckets       map[bucketKey]*Bucket
	dirty         bool
	retentionDays int
	nowFunc       func() time.Time
}

var (
	globalMu    sync.RWMutex
	globalStore *Store
)

// SetGlobal installs the process-wide usage-history store.
func SetGlobal(s *Store) {
	globalMu.Lock()
	globalStore = s
	globalMu.Unlock()
}

// Global returns the process-wide store, or nil when disabled.
func Global() *Store {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalStore
}

// NewStore creates a store backed by the given file path, loading any existing
// rollup from disk. A zero or negative retentionDays uses the default.
func NewStore(path string, retentionDays int) *Store {
	if retentionDays <= 0 {
		retentionDays = defaultRetentionDays
	}
	s := &Store{
		path:          path,
		buckets:       make(map[bucketKey]*Bucket),
		retentionDays: retentionDays,
		nowFunc:       time.Now,
	}
	s.load()
	return s
}

func (s *Store) now() time.Time {
	if s.nowFunc != nil {
		return s.nowFunc()
	}
	return time.Now()
}

// load reads the rollup file into memory. Missing or unreadable files start empty.
func (s *Store) load() {
	if s.path == "" {
		return
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warnf("usagehistory: failed to read %s: %v", s.path, err)
		}
		return
	}
	var snap snapshot
	if errUnmarshal := json.Unmarshal(data, &snap); errUnmarshal != nil {
		log.Warnf("usagehistory: failed to parse %s: %v", s.path, errUnmarshal)
		return
	}
	for i := range snap.Buckets {
		b := snap.Buckets[i]
		key := bucketKey{Hour: b.Hour, Provider: b.Provider, Model: b.Model}
		copyBucket := b
		s.buckets[key] = &copyBucket
	}
}

// Add folds a usage sample into the appropriate hourly bucket and records its
// estimated cost. It is safe for concurrent use.
func (s *Store) Add(sample Sample) {
	if s == nil {
		return
	}
	ts := sample.Timestamp
	if ts.IsZero() {
		ts = s.now()
	}
	hour := ts.UTC().Truncate(time.Hour).Unix()
	provider := sample.Provider
	if provider == "" {
		provider = "unknown"
	}
	model := sample.Model
	if model == "" {
		model = "unknown"
	}

	var cost float64
	if rate, ok := rateFor(model); ok {
		cost = estimateCost(rate, sample.UncachedInput, sample.CacheRead, sample.CacheWrite, sample.OutputTokens)
	}

	key := bucketKey{Hour: hour, Provider: provider, Model: model}

	s.mu.Lock()
	bucket, ok := s.buckets[key]
	if !ok {
		bucket = &Bucket{Hour: hour, Provider: provider, Model: model}
		s.buckets[key] = bucket
	}
	bucket.Requests++
	if sample.Failed {
		bucket.Failed++
	}
	bucket.InputTokens += sample.UncachedInput + sample.CacheRead + sample.CacheWrite
	bucket.OutputTokens += sample.OutputTokens
	bucket.CachedTokens += sample.CachedTokens
	bucket.TotalTokens += sample.TotalTokens
	bucket.CostUSD += cost
	s.dirty = true
	s.mu.Unlock()
}

// Snapshot returns a copy of all buckets, sorted by hour then provider/model.
func (s *Store) Snapshot() []Bucket {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	out := make([]Bucket, 0, len(s.buckets))
	for _, b := range s.buckets {
		out = append(out, *b)
	}
	s.mu.Unlock()

	sort.Slice(out, func(i, j int) bool {
		if out[i].Hour != out[j].Hour {
			return out[i].Hour < out[j].Hour
		}
		if out[i].Provider != out[j].Provider {
			return out[i].Provider < out[j].Provider
		}
		return out[i].Model < out[j].Model
	})
	return out
}

// pruneLocked drops buckets older than the retention window. Caller holds s.mu.
func (s *Store) pruneLocked() {
	if s.retentionDays <= 0 {
		return
	}
	cutoff := s.now().UTC().Add(-time.Duration(s.retentionDays) * 24 * time.Hour).Unix()
	for key, b := range s.buckets {
		if b.Hour < cutoff {
			delete(s.buckets, key)
			s.dirty = true
		}
	}
}

// Flush persists the current rollup to disk when there are pending changes.
// It prunes expired buckets first. Writes are atomic (temp file + rename).
func (s *Store) Flush() error {
	if s == nil || s.path == "" {
		return nil
	}
	s.mu.Lock()
	s.pruneLocked()
	if !s.dirty {
		s.mu.Unlock()
		return nil
	}
	snap := snapshot{
		SchemaVersion: schemaVersion,
		UpdatedAt:     s.now().UTC(),
		Buckets:       make([]Bucket, 0, len(s.buckets)),
	}
	for _, b := range s.buckets {
		snap.Buckets = append(snap.Buckets, *b)
	}
	s.dirty = false
	s.mu.Unlock()

	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	if errMkdir := os.MkdirAll(filepath.Dir(s.path), 0o755); errMkdir != nil {
		return errMkdir
	}
	tmp := s.path + ".tmp"
	if errWrite := os.WriteFile(tmp, data, 0o600); errWrite != nil {
		return errWrite
	}
	return os.Rename(tmp, s.path)
}

// Clear removes all buckets in memory and persists the empty rollup.
func (s *Store) Clear() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	s.buckets = make(map[bucketKey]*Bucket)
	s.dirty = true
	s.mu.Unlock()
	return s.Flush()
}

// StartAutoFlush launches a background goroutine that flushes on the given
// interval until stop is closed, then performs a final flush.
func (s *Store) StartAutoFlush(interval time.Duration, stop <-chan struct{}) {
	if s == nil {
		return
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := s.Flush(); err != nil {
					log.Warnf("usagehistory: flush failed: %v", err)
				}
			case <-stop:
				if err := s.Flush(); err != nil {
					log.Warnf("usagehistory: final flush failed: %v", err)
				}
				return
			}
		}
	}()
}
