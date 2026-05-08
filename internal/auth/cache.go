package auth

import (
	"container/list"
	"sync"
	"time"
)

const (
	DefaultCacheSize = 1000
	DefaultCacheTTL  = 5 * time.Minute
)

type Cache struct {
	mu      sync.Mutex
	max     int
	ttl     time.Duration
	now     func() time.Time
	items   map[string]*list.Element
	entries *list.List
}

type cacheEntry struct {
	hash      string
	key       BridgeKey
	expiresAt time.Time
}

func NewCache(max int, ttl time.Duration, now func() time.Time) *Cache {
	if max <= 0 {
		max = DefaultCacheSize
	}
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}
	if now == nil {
		now = time.Now
	}
	return &Cache{
		max:     max,
		ttl:     ttl,
		now:     now,
		items:   map[string]*list.Element{},
		entries: list.New(),
	}
}

func (c *Cache) Get(hash string) (BridgeKey, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	element, ok := c.items[hash]
	if !ok {
		return BridgeKey{}, false
	}
	entry := element.Value.(cacheEntry)
	if !c.now().Before(entry.expiresAt) {
		c.entries.Remove(element)
		delete(c.items, hash)
		return BridgeKey{}, false
	}
	c.entries.MoveToFront(element)
	return entry.key, true
}

func (c *Cache) Put(hash string, key BridgeKey) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, ok := c.items[hash]; ok {
		element.Value = cacheEntry{hash: hash, key: key, expiresAt: c.now().Add(c.ttl)}
		c.entries.MoveToFront(element)
		return
	}
	element := c.entries.PushFront(cacheEntry{hash: hash, key: key, expiresAt: c.now().Add(c.ttl)})
	c.items[hash] = element
	for c.entries.Len() > c.max {
		oldest := c.entries.Back()
		if oldest == nil {
			return
		}
		entry := oldest.Value.(cacheEntry)
		delete(c.items, entry.hash)
		c.entries.Remove(oldest)
	}
}

func (c *Cache) Invalidate(hash string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	element, ok := c.items[hash]
	if !ok {
		return
	}
	delete(c.items, hash)
	c.entries.Remove(element)
}

func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = map[string]*list.Element{}
	c.entries.Init()
}
