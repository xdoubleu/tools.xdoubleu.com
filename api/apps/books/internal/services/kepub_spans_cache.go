package services

import (
	"container/list"
	"sync"

	"github.com/google/uuid"
)

// spanMapKey identifies a stored KEPUB and the original its offsets point
// into.
type spanMapKey struct {
	kepubID  uuid.UUID
	sourceID uuid.UUID
	version  int16
}

// spanMapCache is a fixed-size LRU of span maps.
type spanMapCache struct {
	mu    sync.Mutex
	size  int
	order *list.List // front is most recently used
	items map[spanMapKey]*list.Element
}

type spanMapEntry struct {
	key spanMapKey
	m   *spanMap
}

func newSpanMapCache(size int) *spanMapCache {
	return &spanMapCache{
		mu:    sync.Mutex{},
		size:  size,
		order: list.New(),
		items: make(map[spanMapKey]*list.Element, size),
	}
}

func (c *spanMapCache) get(key spanMapKey) (*spanMap, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(el)
	return entryOf(el).m, true
}

func entryOf(el *list.Element) *spanMapEntry {
	e, _ := el.Value.(*spanMapEntry)
	return e
}

func (c *spanMapCache) add(key spanMapKey, m *spanMap) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		entryOf(el).m = m
		c.order.MoveToFront(el)
		return
	}
	c.items[key] = c.order.PushFront(&spanMapEntry{key: key, m: m})
	if c.order.Len() > c.size {
		oldest := c.order.Back()
		c.order.Remove(oldest)
		delete(c.items, entryOf(oldest).key)
	}
}
