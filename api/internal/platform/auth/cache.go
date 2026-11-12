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

// cacheSweepInterval is how often set drops every expired entry.
const cacheSweepInterval = 10 * time.Minute

// userCache maps access tokens to enriched users; TTL <= 0 disables it.
// get expires an entry lazily; set also sweeps every cacheSweepInterval,
// since a rotated token is never read again. The map only grows on set, so
// sweeping there bounds it without a goroutine.
type userCache struct {
	mu        sync.Mutex
	ttl       time.Duration
	entries   map[string]cacheEntry
	now       func() time.Time
	lastSweep time.Time
}

func newUserCache(ttl time.Duration) *userCache {
	return &userCache{
		mu:        sync.Mutex{},
		ttl:       ttl,
		entries:   make(map[string]cacheEntry),
		now:       time.Now,
		lastSweep: time.Now(),
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
	if c.now().After(entry.expiresAt) {
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

	now := c.now()
	if now.Sub(c.lastSweep) >= cacheSweepInterval {
		c.lastSweep = now
		for t, entry := range c.entries {
			if now.After(entry.expiresAt) {
				delete(c.entries, t)
			}
		}
	}
	c.entries[token] = cacheEntry{
		user:      user,
		expiresAt: now.Add(c.ttl),
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
