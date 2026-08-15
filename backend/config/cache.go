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
	mu       sync.RWMutex
	items    map[string]CacheEntry
	stop     chan struct{}
	stopOnce sync.Once
}

// NewInMemoryCache создаёт новый кэш. Если передан положительный интервал,
// просроченные записи дополнительно удаляются фоновым сборщиком.
func NewInMemoryCache(cleanupInterval ...time.Duration) *InMemoryCache {
	c := &InMemoryCache{
		items: make(map[string]CacheEntry),
		stop:  make(chan struct{}),
	}
	if len(cleanupInterval) > 0 && cleanupInterval[0] > 0 {
		go c.cleanupLoop(cleanupInterval[0])
	}
	return c
}

// Get возвращает значение из кэша, если оно не истекло.
func (c *InMemoryCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.items[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(c.items, key)
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

func (c *InMemoryCache) cleanupExpired(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, entry := range c.items {
		if now.After(entry.ExpiresAt) {
			delete(c.items, key)
		}
	}
}

func (c *InMemoryCache) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case now := <-ticker.C:
			c.cleanupExpired(now)
		case <-c.stop:
			return
		}
	}
}

// Close останавливает фоновый сборщик. Глобальный кэш живёт до остановки процесса;
// метод нужен владельцам временных кэшей и тестам.
func (c *InMemoryCache) Close() {
	c.stopOnce.Do(func() { close(c.stop) })
}

func (c *InMemoryCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// FiltersCache — глобальный кэш для значений фильтров.
// Инициализируется автоматически при старте.
var FiltersCache *InMemoryCache

func init() {
	FiltersCache = NewInMemoryCache(10 * time.Minute)
}

// FilterCacheTTL — 5 минут.
const FilterCacheTTL = 5 * time.Minute
