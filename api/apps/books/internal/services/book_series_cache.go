package services

import (
	"sync"
	"time"

	"tools.xdoubleu.com/apps/books/pkg/hardcover"
)

// seriesCacheTTL spares the rate-limited Hardcover client a page's SSR fetch
// plus its client revalidation, and repeat visits.
const seriesCacheTTL = 10 * time.Minute

// seriesCache holds Hardcover series lookups by name; nil (unknown) results
// are cached too. A nil cache never hits.
type seriesCache struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]seriesCacheEntry
}

type seriesCacheEntry struct {
	series  *hardcover.Series
	fetched time.Time
}

func newSeriesCache() *seriesCache {
	return &seriesCache{
		mu:      sync.Mutex{},
		now:     time.Now,
		entries: map[string]seriesCacheEntry{},
	}
}

func (c *seriesCache) get(name string) (*hardcover.Series, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[name]
	if !ok || c.now().Sub(e.fetched) > seriesCacheTTL {
		delete(c.entries, name)
		return nil, false
	}
	return e.series, true
}

func (c *seriesCache) put(name string, series *hardcover.Series) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[name] = seriesCacheEntry{series: series, fetched: c.now()}
}
