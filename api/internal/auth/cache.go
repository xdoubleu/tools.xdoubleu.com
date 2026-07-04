package auth

import (
	"sync"
	"time"

	"tools.xdoubleu.com/internal/models"
)

type cacheEntry struct {
	user      models.User
	expiresAt time.Time
}

// userCache is a TTL cache mapping access tokens to DB-enriched users so the
// auth middleware can skip the GoTrue round-trip and enrichment queries on
// every request. A zero (or negative) TTL disables it. Expiry is lazy: stale
// entries are dropped on read, and the cache only ever holds active sessions,
// so no background sweeper is needed.
type userCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]cacheEntry
}

func newUserCache(ttl time.Duration) *userCache {
	return &userCache{
		mu:      sync.Mutex{},
		ttl:     ttl,
		entries: make(map[string]cacheEntry),
	}
}

func (c *userCache) get(token string) (models.User, bool) {
	var zero models.User

	if c.ttl <= 0 {
		return zero, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[token]
	if !ok {
		return zero, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.entries, token)
		return zero, false
	}
	return entry.user, true
}

func (c *userCache) set(token string, user models.User) {
	if c.ttl <= 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[token] = cacheEntry{
		user:      user,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *userCache) evict(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, token)
}

func (c *userCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	clear(c.entries)
}
