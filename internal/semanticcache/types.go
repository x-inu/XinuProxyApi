// Package semanticcache provides an exact-match prompt/response cache for
// non-streaming OpenAI-compatible requests. It is adapted from XRoute's
// semantic cache, narrowed to v7: keys are derived from the raw JSON request
// bytes (model, messages, tools, temperature, token limits) with no translator
// dependency. The cache is inert unless `semantic-cache.enabled` is set.
package semanticcache

import "time"

// Entry represents a cached response body.
type Entry struct {
	Key          string
	Model        string
	ResponseBody []byte
	ContentType  string
	StoredAt     time.Time
	HitCount     int64
	TokensSaved  int64
}

// Stats represents overall semantic cache metrics.
type Stats struct {
	MemoryEntries int    `json:"memoryEntries"`
	DBEntries     int    `json:"dbEntries"`
	Hits          int64  `json:"hits"`
	Misses        int64  `json:"misses"`
	HitRate       string `json:"hitRate"`
	TokensSaved   int64  `json:"tokensSaved"`
}

// Config configures semantic cache behavior.
type Config struct {
	Enabled    bool
	TTL        time.Duration
	MaxEntries int
}
