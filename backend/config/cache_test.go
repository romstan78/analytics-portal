package config

import (
	"testing"
	"time"
)

func TestInMemoryCacheDeletesExpiredEntryOnGet(t *testing.T) {
	cache := NewInMemoryCache()
	defer cache.Close()
	cache.Set("expired", "value", -time.Second)

	if _, ok := cache.Get("expired"); ok {
		t.Fatal("expired entry must not be returned")
	}
	if got := cache.Len(); got != 0 {
		t.Fatalf("cache length = %d, want 0", got)
	}
}

func TestInMemoryCacheBackgroundCleanup(t *testing.T) {
	cache := NewInMemoryCache(5 * time.Millisecond)
	defer cache.Close()
	cache.Set("expired", "value", time.Millisecond)

	deadline := time.Now().Add(250 * time.Millisecond)
	for cache.Len() != 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := cache.Len(); got != 0 {
		t.Fatalf("cache length after cleanup = %d, want 0", got)
	}
}
