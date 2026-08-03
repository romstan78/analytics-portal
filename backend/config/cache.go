package config

import (
	"sync"
	"time"
)

// CacheEntry — одна запись в кэше.
type CacheEntry struct {
	Data      interface{}
	ExpiresAt time.Time
}

// InMemoryCache — простой потокобезопасный кэш с TTL.
type InMemoryCache struct {
	mu    sync.RWMutex
	items map[string]CacheEntry
}

// NewInMemoryCache создаёт новый кэш.
func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{
		items: make(map[string]CacheEntry),
	}
}

// Get возвращает значение из кэша, если оно не истекло.
func (c *InMemoryCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.items[key]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return nil, false
	}
	return entry.Data, true
}

// Set сохраняет значение в кэш с указанным TTL.
func (c *InMemoryCache) Set(key string, data interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = CacheEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(ttl),
	}
}

// FiltersCache — глобальный кэш для значений фильтров.
// Инициализируется автоматически при старте.
var FiltersCache *InMemoryCache

func init() {
	FiltersCache = NewInMemoryCache()
}

// FilterCacheTTL — 5 минут.
const FilterCacheTTL = 5 * time.Minute
