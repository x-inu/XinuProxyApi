package semanticcache

import (
	"container/list"
	"context"
	"sync"
	"time"
)

// EntryStore is the storage interface for cached entries.
type EntryStore interface {
	Get(ctx context.Context, key string) (Entry, bool)
	Put(ctx context.Context, key string, e Entry) error
	Delete(ctx context.Context, key string) bool
	Len() int
	Entries() []Entry
	InvalidateByModel(ctx context.Context, model string) int
	RecordHit(ctx context.Context, key string, tokensSaved int64)
	Clear()
}

type lruItem struct {
	key   string
	entry Entry
}

// LRUStore is a thread-safe, bounded in-memory LRU store with TTL.
type LRUStore struct {
	mu         sync.Mutex
	items      map[string]*list.Element
	evictList  *list.List
	maxEntries int
	ttl        time.Duration
}

// NewLRUStore creates a bounded LRUStore.
func NewLRUStore(maxEntries int, ttl time.Duration) *LRUStore {
	if maxEntries <= 0 {
		maxEntries = 1000
	}
	return &LRUStore{
		items:      make(map[string]*list.Element),
		evictList:  list.New(),
		maxEntries: maxEntries,
		ttl:        ttl,
	}
}

// Get returns the cached entry, or false if missing or expired.
func (s *LRUStore) Get(_ context.Context, key string) (Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	elem, ok := s.items[key]
	if !ok {
		return Entry{}, false
	}
	item := elem.Value.(*lruItem)
	if s.ttl > 0 && time.Since(item.entry.StoredAt) > s.ttl {
		s.removeElement(elem)
		return Entry{}, false
	}
	s.evictList.MoveToFront(elem)
	clone := item.entry
	clone.ResponseBody = CloneBody(item.entry.ResponseBody)
	return clone, true
}

// Put stores or updates an entry, evicting LRU entries if full.
func (s *LRUStore) Put(_ context.Context, key string, e Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e.ResponseBody = CloneBody(e.ResponseBody)
	if e.StoredAt.IsZero() {
		e.StoredAt = time.Now()
	}

	if elem, ok := s.items[key]; ok {
		s.evictList.MoveToFront(elem)
		elem.Value.(*lruItem).entry = e
		return nil
	}

	for s.evictList.Len() >= s.maxEntries {
		s.removeOldest()
	}
	elem := s.evictList.PushFront(&lruItem{key: key, entry: e})
	s.items[key] = elem
	return nil
}

// Delete removes an entry by key.
func (s *LRUStore) Delete(_ context.Context, key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	elem, ok := s.items[key]
	if !ok {
		return false
	}
	s.removeElement(elem)
	return true
}

// RecordHit increments hit counter and tokens saved.
func (s *LRUStore) RecordHit(_ context.Context, key string, tokensSaved int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if elem, ok := s.items[key]; ok {
		item := elem.Value.(*lruItem)
		item.entry.HitCount++
		item.entry.TokensSaved += tokensSaved
	}
}

// Entries returns active entries.
func (s *LRUStore) Entries() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	res := make([]Entry, 0, len(s.items))
	for _, elem := range s.items {
		item := elem.Value.(*lruItem)
		if s.ttl > 0 && now.Sub(item.entry.StoredAt) > s.ttl {
			continue
		}
		clone := item.entry
		clone.ResponseBody = CloneBody(item.entry.ResponseBody)
		res = append(res, clone)
	}
	return res
}

// InvalidateByModel removes entries by model.
func (s *LRUStore) InvalidateByModel(_ context.Context, model string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	var toRemove []*list.Element
	count := 0
	for _, elem := range s.items {
		item := elem.Value.(*lruItem)
		if item.entry.Model == model {
			toRemove = append(toRemove, elem)
		}
	}
	for _, elem := range toRemove {
		s.removeElement(elem)
		count++
	}
	return count
}

// Len returns the number of cached entries.
func (s *LRUStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.items)
}

// Clear flushes all entries.
func (s *LRUStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = make(map[string]*list.Element)
	s.evictList.Init()
}

func (s *LRUStore) removeElement(elem *list.Element) {
	s.evictList.Remove(elem)
	item := elem.Value.(*lruItem)
	delete(s.items, item.key)
}

func (s *LRUStore) removeOldest() {
	elem := s.evictList.Back()
	if elem != nil {
		s.removeElement(elem)
	}
}
