package services

import (
	"container/list"
	"context"
	"io"
	"sync"

	"tools.xdoubleu.com/apps/books/pkg/ebooks"
)

// webPubArchiveSlots bounds the EPUBs the web reader keeps open. Each is a
// temp file, so a slot costs disk, not memory.
const webPubArchiveSlots = 4

// archiveCache shares opened stored zips between requests by storage key, so
// a reading session downloads its EPUB once. An evicted zip is closed once
// its last user releases it.
type archiveCache struct {
	mu    sync.Mutex
	size  int
	order *list.List // of *archive; front is most recently used
	items map[string]*list.Element
	open  func(ctx context.Context, key string) (*ebooks.TempZip, error)
}

type archive struct {
	key     string
	ready   chan struct{} // closed once zip or err is set
	zip     *ebooks.TempZip
	err     error
	refs    int
	evicted bool
}

func newArchiveCache(
	size int,
	open func(ctx context.Context, key string) (*ebooks.TempZip, error),
) *archiveCache {
	return &archiveCache{
		mu:    sync.Mutex{},
		size:  size,
		order: list.New(),
		items: make(map[string]*list.Element, size),
		open:  open,
	}
}

// acquire returns key's opened zip, opening it at most once however many
// requests ask concurrently. Callers must release it.
func (c *archiveCache) acquire(ctx context.Context, key string) (*archive, error) {
	c.mu.Lock()
	el, hit := c.items[key]
	var a *archive
	if hit {
		a, _ = el.Value.(*archive)
		c.order.MoveToFront(el)
	} else {
		//nolint:exhaustruct // zip and err are set once opened
		a = &archive{key: key, ready: make(chan struct{})}
		c.items[key] = c.order.PushFront(a)
		c.evictLocked()
	}
	a.refs++
	c.mu.Unlock()

	if !hit {
		a.zip, a.err = c.open(ctx, key)
		if a.err != nil {
			c.drop(a)
		}
		close(a.ready)
	}
	select {
	case <-a.ready:
	case <-ctx.Done():
		c.release(a)
		return nil, ctx.Err()
	}
	if a.err != nil {
		c.release(a)
		return nil, a.err
	}
	return a, nil
}

func (c *archiveCache) release(a *archive) {
	c.mu.Lock()
	defer c.mu.Unlock()
	a.refs--
	if a.refs == 0 && a.evicted && a.zip != nil {
		a.zip.Close()
	}
}

// drop forgets a, so a failed open is retried by the next request.
func (c *archiveCache) drop(a *archive) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[a.key]; ok && el.Value == a {
		c.order.Remove(el)
		delete(c.items, a.key)
	}
	a.evicted = true
}

func (c *archiveCache) evictLocked() {
	for c.order.Len() > c.size {
		oldest, _ := c.order.Remove(c.order.Back()).(*archive)
		delete(c.items, oldest.key)
		oldest.evicted = true
		if oldest.refs == 0 && oldest.zip != nil {
			oldest.zip.Close()
		}
	}
}

// releasingBody runs release once the body is closed.
type releasingBody struct {
	io.ReadCloser
	release func()
	once    sync.Once
}

func (b *releasingBody) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.release)
	return err
}
