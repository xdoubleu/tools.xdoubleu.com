package positions

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

type spanMapCache = lruCache[spanMapKey, *spanMap]

func newSpanMapCache(size int) *spanMapCache {
	return newLRUCache[spanMapKey, *spanMap](size)
}

// lruCache is a fixed-size LRU.
type lruCache[K comparable, V any] struct {
	mu    sync.Mutex
	size  int
	order *list.List // front is most recently used
	items map[K]*list.Element
}

type lruEntry[K comparable, V any] struct {
	key K
	v   V
}

func newLRUCache[K comparable, V any](size int) *lruCache[K, V] {
	return &lruCache[K, V]{
		mu:    sync.Mutex{},
		size:  size,
		order: list.New(),
		items: make(map[K]*list.Element, size),
	}
}

func (c *lruCache[K, V]) get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	c.order.MoveToFront(el)
	return entryOf[K, V](el).v, true
}

func entryOf[K comparable, V any](el *list.Element) *lruEntry[K, V] {
	e, _ := el.Value.(*lruEntry[K, V])
	return e
}

func (c *lruCache[K, V]) add(key K, v V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		entryOf[K, V](el).v = v
		c.order.MoveToFront(el)
		return
	}
	c.items[key] = c.order.PushFront(&lruEntry[K, V]{key: key, v: v})
	if c.order.Len() > c.size {
		oldest := c.order.Back()
		c.order.Remove(oldest)
		delete(c.items, entryOf[K, V](oldest).key)
	}
}
