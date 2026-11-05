// Package usagehistory provides a lightweight, persistent rollup of token
// usage and estimated cost. It aggregates per-request usage records into
// hourly buckets keyed by provider and model, keeping memory and storage use
// minimal. It supports both atomic JSON snapshots and high-performance SQLite WAL storage.
package usagehistory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
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
	sqliteDB      *sql.DB
	ownsDB        bool
	isSqlite      bool
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

// isSqliteFile checks if a file path points to a SQLite database.
func isSqliteFile(path string) bool {
	lower := strings.ToLower(strings.TrimSpace(path))
	return strings.HasSuffix(lower, ".sqlite") || strings.HasSuffix(lower, ".db")
}

// NewStore creates a store backed by the given file path.
// If the path ends with .sqlite or .db, it automatically opens SQLite WAL mode;
// otherwise it uses the JSON snapshot format. A zero or negative retentionDays uses default.
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

	if isSqliteFile(path) {
		s.isSqlite = true
		if err := s.initSqlite(path); err != nil {
			log.Errorf("usagehistory: failed to initialize sqlite database at %s: %v", path, err)
		}
	} else {
		s.load()
	}
	return s
}

// NewStoreWithDB creates a store backed by an existing SQLite *sql.DB connection.
func NewStoreWithDB(db *sql.DB, retentionDays int) *Store {
	if retentionDays <= 0 {
		retentionDays = defaultRetentionDays
	}
	s := &Store{
		sqliteDB:      db,
		isSqlite:      true,
		ownsDB:        false,
		buckets:       make(map[bucketKey]*Bucket),
		retentionDays: retentionDays,
		nowFunc:       time.Now,
	}
	if err := s.initSqliteSchema(); err != nil {
		log.Errorf("usagehistory: failed to initialize sqlite schema: %v", err)
	}
	s.loadSqlite()
	return s
}

func (s *Store) initSqlite(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create sqlite dir %s: %w", dir, err)
	}
	_ = os.Chmod(dir, 0o700)

	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}

	if _, err = db.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		_ = db.Close()
		return fmt.Errorf("set WAL mode: %w", err)
	}
	_ = os.Chmod(path, 0o600)

	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(time.Hour)

	s.sqliteDB = db
	s.ownsDB = true

	if err := s.initSqliteSchema(); err != nil {
		return err
	}
	s.migrateFromJSONIfEmpty(path)
	s.loadSqlite()
	return nil
}

func (s *Store) migrateFromJSONIfEmpty(sqlitePath string) {
	if s.sqliteDB == nil {
		return
	}
	var count int
	err := s.sqliteDB.QueryRow("SELECT COUNT(1) FROM usage_history").Scan(&count)
	if err != nil || count > 0 {
		return
	}

	dir := filepath.Dir(sqlitePath)
	candidates := []string{
		strings.TrimSuffix(sqlitePath, filepath.Ext(sqlitePath)) + ".json",
		filepath.Join(dir, "usage-history.json"),
	}
	if cwd, err := os.Getwd(); err == nil && cwd != dir {
		candidates = append(candidates, filepath.Join(cwd, "usage-history.json"))
	}

	for _, jsonFile := range candidates {
		data, err := os.ReadFile(jsonFile)
		if err != nil || len(data) == 0 {
			continue
		}
		var snap snapshot
		if errUnmarshal := json.Unmarshal(data, &snap); errUnmarshal != nil || len(snap.Buckets) == 0 {
			continue
		}

		log.Infof("usagehistory: migrating %d legacy usage buckets from %s to sqlite", len(snap.Buckets), jsonFile)
		tx, errTx := s.sqliteDB.Begin()
		if errTx != nil {
			return
		}
		stmt, errStmt := tx.Prepare(`INSERT INTO usage_history (hour, provider, model, requests, failed, input_tokens, output_tokens, cached_tokens, total_tokens, cost_usd)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(hour, provider, model) DO UPDATE SET
				requests = excluded.requests,
				failed = excluded.failed,
				input_tokens = excluded.input_tokens,
				output_tokens = excluded.output_tokens,
				cached_tokens = excluded.cached_tokens,
				total_tokens = excluded.total_tokens,
				cost_usd = excluded.cost_usd`)
		if errStmt != nil {
			_ = tx.Rollback()
			return
		}

		for _, b := range snap.Buckets {
			_, _ = stmt.Exec(b.Hour, b.Provider, b.Model, b.Requests, b.Failed, b.InputTokens, b.OutputTokens, b.CachedTokens, b.TotalTokens, b.CostUSD)
		}
		_ = stmt.Close()
		_ = tx.Commit()
		break
	}
}

func (s *Store) initSqliteSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS usage_history (
		hour INTEGER NOT NULL,
		provider TEXT NOT NULL,
		model TEXT NOT NULL,
		requests INTEGER NOT NULL DEFAULT 0,
		failed INTEGER NOT NULL DEFAULT 0,
		input_tokens INTEGER NOT NULL DEFAULT 0,
		output_tokens INTEGER NOT NULL DEFAULT 0,
		cached_tokens INTEGER NOT NULL DEFAULT 0,
		total_tokens INTEGER NOT NULL DEFAULT 0,
		cost_usd REAL NOT NULL DEFAULT 0,
		PRIMARY KEY (hour, provider, model)
	);
	CREATE INDEX IF NOT EXISTS idx_usage_history_hour ON usage_history(hour);
	CREATE INDEX IF NOT EXISTS idx_usage_history_provider ON usage_history(provider);
	CREATE INDEX IF NOT EXISTS idx_usage_history_model ON usage_history(model);
	`
	_, err := s.sqliteDB.Exec(schema)
	return err
}

func (s *Store) loadSqlite() {
	if s.sqliteDB == nil {
		return
	}
	cutoff := int64(0)
	if s.retentionDays > 0 {
		cutoff = s.now().UTC().Add(-time.Duration(s.retentionDays) * 24 * time.Hour).Unix()
	}

	rows, err := s.sqliteDB.Query(`SELECT hour, provider, model, requests, failed, input_tokens, output_tokens, cached_tokens, total_tokens, cost_usd 
		FROM usage_history WHERE hour >= ?`, cutoff)
	if err != nil {
		log.Warnf("usagehistory: sqlite query failed: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var b Bucket
		if errScan := rows.Scan(&b.Hour, &b.Provider, &b.Model, &b.Requests, &b.Failed, &b.InputTokens, &b.OutputTokens, &b.CachedTokens, &b.TotalTokens, &b.CostUSD); errScan != nil {
			log.Warnf("usagehistory: sqlite scan failed: %v", errScan)
			return
		}
		key := bucketKey{Hour: b.Hour, Provider: b.Provider, Model: b.Model}
		s.buckets[key] = &b
	}
}

func (s *Store) now() time.Time {
	if s.nowFunc != nil {
		return s.nowFunc()
	}
	return time.Now()
}

// load reads the JSON rollup file into memory. Missing or unreadable files start empty.
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
	if s.isSqlite && s.sqliteDB != nil {
		_, _ = s.sqliteDB.Exec("DELETE FROM usage_history WHERE hour < ?", cutoff)
	}
}

// Flush persists the current rollup to SQLite or JSON disk file.
func (s *Store) Flush() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	s.pruneLocked()
	if !s.dirty {
		s.mu.Unlock()
		return nil
	}

	if s.isSqlite && s.sqliteDB != nil {
		// Flush to SQLite
		tx, err := s.sqliteDB.Begin()
		if err != nil {
			s.mu.Unlock()
			return err
		}
		defer tx.Rollback()

		stmt, err := tx.Prepare(`INSERT INTO usage_history (hour, provider, model, requests, failed, input_tokens, output_tokens, cached_tokens, total_tokens, cost_usd)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(hour, provider, model) DO UPDATE SET
				requests = excluded.requests,
				failed = excluded.failed,
				input_tokens = excluded.input_tokens,
				output_tokens = excluded.output_tokens,
				cached_tokens = excluded.cached_tokens,
				total_tokens = excluded.total_tokens,
				cost_usd = excluded.cost_usd`)
		if err != nil {
			s.mu.Unlock()
			return err
		}
		defer stmt.Close()

		for _, b := range s.buckets {
			if _, errExec := stmt.Exec(b.Hour, b.Provider, b.Model, b.Requests, b.Failed, b.InputTokens, b.OutputTokens, b.CachedTokens, b.TotalTokens, b.CostUSD); errExec != nil {
				s.mu.Unlock()
				return errExec
			}
		}

		if errCommit := tx.Commit(); errCommit != nil {
			s.mu.Unlock()
			return errCommit
		}
		s.dirty = false
		s.mu.Unlock()
		return nil
	}

	// JSON format flush
	if s.path == "" {
		s.dirty = false
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
	if s.isSqlite && s.sqliteDB != nil {
		_, _ = s.sqliteDB.Exec("DELETE FROM usage_history")
	}
	s.mu.Unlock()
	return s.Flush()
}

// Close closes the underlying SQLite database if opened by this store.
func (s *Store) Close() error {
	if s == nil || !s.ownsDB || s.sqliteDB == nil {
		return nil
	}
	return s.sqliteDB.Close()
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
					log.Warnf("usagehistory: flush: %v", err)
				}
			case <-stop:
				_ = s.Flush()
				return
			}
		}
	}()
}
