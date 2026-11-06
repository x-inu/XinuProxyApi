package semanticcache

import (
	"sync"
)

var (
	globalMu    sync.RWMutex
	globalCache *Cache
)

// SetGlobal installs the process-wide semantic cache instance.
func SetGlobal(c *Cache) {
	globalMu.Lock()
	globalCache = c
	globalMu.Unlock()
}

// Global returns the process-wide semantic cache instance, or nil when not set.
func Global() *Cache {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalCache
}
