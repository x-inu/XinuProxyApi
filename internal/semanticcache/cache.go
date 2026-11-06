package semanticcache

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

// Cache coordinates exact-match prompt/response caching.
type Cache struct {
	cfg         Config
	store       EntryStore
	hits        atomic.Int64
	misses      atomic.Int64
	tokensSaved atomic.Int64
}

// New creates a Cache with the given config and store.
func New(cfg Config, store EntryStore) *Cache {
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = 1000
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 24 * time.Hour
	}
	if store == nil {
		store = NewLRUStore(cfg.MaxEntries, cfg.TTL)
	}
	return &Cache{cfg: cfg, store: store}
}

// Enabled reports whether caching is active.
func (c *Cache) Enabled() bool {
	if c == nil {
		return false
	}
	return c.cfg.Enabled
}

// TTL returns the configured TTL.
func (c *Cache) TTL() time.Duration {
	if c == nil {
		return 0
	}
	return c.cfg.TTL
}

// MaxEntries returns the configured capacity.
func (c *Cache) MaxEntries() int {
	if c == nil {
		return 0
	}
	return c.cfg.MaxEntries
}

// Lookup queries the cache for a non-streaming request body.
func (c *Cache) Lookup(ctx context.Context, sessionID string, body []byte) (*Entry, bool) {
	if !c.Enabled() {
		return nil, false
	}
	key := BuildCacheKey(sessionID, body)
	if key == "" {
		c.misses.Add(1)
		return nil, false
	}

	entry, ok := c.store.Get(ctx, key)
	if !ok {
		c.misses.Add(1)
		return nil, false
	}

	c.hits.Add(1)
	saved := int64(EstimateTokens(entry.ResponseBody))
	c.tokensSaved.Add(saved)
	c.store.RecordHit(ctx, key, saved)
	return &entry, true
}

// Store saves a successful non-streaming response.
func (c *Cache) Store(ctx context.Context, sessionID string, body []byte, responseBody []byte, contentType string) error {
	if !c.Enabled() {
		return nil
	}
	if len(responseBody) == 0 {
		return nil
	}
	key := BuildCacheKey(sessionID, body)
	if key == "" {
		return nil
	}
	model := ""
	if m := stringFromBody(body); m != "" {
		model = m
	}
	if contentType == "" {
		contentType = "application/json"
	}
	return c.store.Put(ctx, key, Entry{
		Key:          key,
		Model:        model,
		ResponseBody: AppendSafe(responseBody),
		ContentType:  contentType,
		StoredAt:     time.Now(),
	})
}

// AppendSafe deep-copies the response body.
func AppendSafe(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// Stats returns overall semantic cache metrics.
func (c *Cache) Stats() Stats {
	if c == nil {
		return Stats{HitRate: "0.0"}
	}
	hits := c.hits.Load()
	misses := c.misses.Load()
	total := hits + misses
	hitRate := "0.0"
	if total > 0 {
		hitRate = fmt.Sprintf("%.1f", float64(hits)/float64(total)*100.0)
	}
	return Stats{
		Hits:          hits,
		Misses:        misses,
		HitRate:       hitRate,
		TokensSaved:   c.tokensSaved.Load(),
		MemoryEntries: c.storeLen(),
		DBEntries:     0,
	}
}

func (c *Cache) storeLen() int {
	if c.store == nil {
		return 0
	}
	return c.store.Len()
}

// ListEntries returns a paginated, filtered, sorted list of entry metadata.
func (c *Cache) ListEntries(page, limit int, search, model, sortBy, sortOrder string) ([]EntryMeta, int) {
	if c == nil || c.store == nil {
		return nil, 0
	}
	entries := c.store.Entries()
	filtered := make([]EntryMeta, 0, len(entries))
	searchLower := strings.ToLower(search)
	modelLower := strings.ToLower(model)

	for _, e := range entries {
		if modelLower != "" && strings.ToLower(e.Model) != modelLower {
			continue
		}
		if searchLower != "" && !strings.Contains(strings.ToLower(e.Key), searchLower) {
			continue
		}
		expiresAt := ""
		if c.cfg.TTL > 0 {
			expiresAt = e.StoredAt.Add(c.cfg.TTL).UTC().Format(time.RFC3339)
		}
		filtered = append(filtered, EntryMeta{
			ID:          e.Key,
			Signature:   e.Key,
			Model:       e.Model,
			HitCount:    e.HitCount,
			TokensSaved: e.TokensSaved,
			CreatedAt:   e.StoredAt.UTC().Format(time.RFC3339),
			ExpiresAt:   expiresAt,
		})
	}

	sort.Slice(filtered, func(i, j int) bool {
		switch sortBy {
		case "hits", "hit_count":
			if sortOrder == "asc" {
				return filtered[i].HitCount < filtered[j].HitCount
			}
			return filtered[i].HitCount > filtered[j].HitCount
		case "tokens_saved":
			if sortOrder == "asc" {
				return filtered[i].TokensSaved < filtered[j].TokensSaved
			}
			return filtered[i].TokensSaved > filtered[j].TokensSaved
		case "model":
			if sortOrder == "asc" {
				return filtered[i].Model < filtered[j].Model
			}
			return filtered[i].Model > filtered[j].Model
		default:
			if sortOrder == "asc" {
				return filtered[i].CreatedAt < filtered[j].CreatedAt
			}
			return filtered[i].CreatedAt > filtered[j].CreatedAt
		}
	})

	total := len(filtered)
	if limit <= 0 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	start := (page - 1) * limit
	if start >= total {
		return []EntryMeta{}, total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return filtered[start:end], total
}

// EntryMeta is metadata for a cached entry.
type EntryMeta struct {
	ID          string `json:"id"`
	Signature   string `json:"signature"`
	Model       string `json:"model"`
	HitCount    int64  `json:"hit_count"`
	TokensSaved int64  `json:"tokens_saved"`
	CreatedAt   string `json:"created_at"`
	ExpiresAt   string `json:"expires_at"`
}

// DeleteEntry deletes an entry by key.
func (c *Cache) DeleteEntry(ctx context.Context, key string) bool {
	if c == nil || c.store == nil {
		return false
	}
	return c.store.Delete(ctx, key)
}

// InvalidateByModel deletes entries for a model.
func (c *Cache) InvalidateByModel(ctx context.Context, model string) int {
	if c == nil || c.store == nil {
		return 0
	}
	return c.store.InvalidateByModel(ctx, model)
}

// Clear flushes all entries.
func (c *Cache) Clear() {
	if c == nil || c.store == nil {
		return
	}
	c.store.Clear()
}

// Len returns the number of active cached entries.
func (c *Cache) Len() int {
	if c == nil || c.store == nil {
		return 0
	}
	return c.store.Len()
}

func stringFromBody(body []byte) string {
	var tmp map[string]any
	if err := json.Unmarshal(body, &tmp); err != nil {
		return ""
	}
	if v, ok := tmp["model"].(string); ok {
		return v
	}
	return ""
}

var _ = http.StatusOK // referenced for content-type hint parity
