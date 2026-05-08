package auth

import (
	"testing"
	"time"
)

func TestCacheHitMissExpireInvalidate(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	cache := NewCache(2, time.Minute, func() time.Time { return now })
	cache.Put("a", BridgeKey{ID: "a"})
	if got, ok := cache.Get("a"); !ok || got.ID != "a" {
		t.Fatalf("Get(a) = %#v, %v", got, ok)
	}
	if _, ok := cache.Get("missing"); ok {
		t.Fatal("Get(missing) hit")
	}
	now = now.Add(2 * time.Minute)
	if _, ok := cache.Get("a"); ok {
		t.Fatal("Get(a) hit after expiration")
	}
	cache.Put("b", BridgeKey{ID: "b"})
	cache.Invalidate("b")
	if _, ok := cache.Get("b"); ok {
		t.Fatal("Get(b) hit after invalidate")
	}
}

func TestCacheEvictsLRU(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	cache := NewCache(2, time.Minute, func() time.Time { return now })
	cache.Put("a", BridgeKey{ID: "a"})
	cache.Put("b", BridgeKey{ID: "b"})
	if _, ok := cache.Get("a"); !ok {
		t.Fatal("expected a hit")
	}
	cache.Put("c", BridgeKey{ID: "c"})
	if _, ok := cache.Get("b"); ok {
		t.Fatal("b should have been evicted")
	}
	if _, ok := cache.Get("a"); !ok {
		t.Fatal("a should still be cached")
	}
}
