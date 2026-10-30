//nolint:testpackage // testing the unexported archive cache
package services

import (
	"bytes"
	"context"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/pkg/ebooks"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
)

type countingOpener struct {
	store *objectstore.FakeClient
	opens atomic.Int32
	gate  chan struct{} // nil, or closed to let opens finish
}

func (o *countingOpener) open(
	ctx context.Context,
	key string,
) (*ebooks.TempZip, error) {
	o.opens.Add(1)
	if o.gate != nil {
		<-o.gate
	}
	return ebooks.OpenStoredZip(ctx, o.store, key, 1<<20)
}

func readable(a *archive) bool {
	rc, err := a.zip.Reader().File[0].Open()
	if err != nil {
		return false
	}
	defer func() { _ = rc.Close() }()
	_, err = io.ReadAll(rc)
	return err == nil
}

func TestArchiveCache_ConcurrentRequestsOpenOnce(t *testing.T) {
	//nolint:exhaustruct // opens is zero-value-ready
	o := &countingOpener{store: storedZipFake(t, "k", 16), gate: make(chan struct{})}
	c := newArchiveCache(2, o.open)

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			a, err := c.acquire(t.Context(), "k")
			if assert.NoError(t, err) {
				assert.True(t, readable(a))
				c.release(a)
			}
		})
	}
	close(o.gate)
	wg.Wait()

	assert.Equal(t, int32(1), o.opens.Load())
}

func TestArchiveCache_EvictedArchiveClosesOnLastRelease(t *testing.T) {
	store := storedZipFake(t, "a", 16)
	data, _ := store.GetContent("a")
	require.NoError(t, store.Put(t.Context(), "b", bytes.NewReader(data), 0, ""))
	o := &countingOpener{store: store} //nolint:exhaustruct // no gate
	c := newArchiveCache(1, o.open)

	a, err := c.acquire(t.Context(), "a")
	require.NoError(t, err)
	b, err := c.acquire(t.Context(), "b")
	require.NoError(t, err)

	assert.True(t, readable(a), "an evicted archive in use stays open")
	c.release(a)
	assert.False(t, readable(a), "closed once released")

	c.release(b)
	b, err = c.acquire(t.Context(), "b")
	require.NoError(t, err)
	assert.True(t, readable(b), "a released archive stays cached")
	c.release(b)
	assert.Equal(t, int32(2), o.opens.Load())
}

func TestArchiveCache_FailedOpenIsRetried(t *testing.T) {
	o := &countingOpener{store: objectstore.NewFake()} //nolint:exhaustruct // no gate
	c := newArchiveCache(1, o.open)

	_, err := c.acquire(t.Context(), "k")
	require.ErrorIs(t, err, ebooks.ErrZipDownload)

	o.store = storedZipFake(t, "k", 16)
	a, err := c.acquire(t.Context(), "k")
	require.NoError(t, err)
	c.release(a)
	assert.Equal(t, int32(2), o.opens.Load())
}

func TestArchiveCache_WaiterHonoursItsContext(t *testing.T) {
	//nolint:exhaustruct // opens is zero-value-ready
	o := &countingOpener{store: storedZipFake(t, "k", 16), gate: make(chan struct{})}
	c := newArchiveCache(1, o.open)
	opened := make(chan struct{})
	go func() {
		defer close(opened)
		a, err := c.acquire(context.Background(), "k")
		if assert.NoError(t, err) {
			c.release(a)
		}
	}()
	require.Eventually(t, func() bool { return o.opens.Load() == 1 }, time.Second,
		time.Millisecond)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := c.acquire(ctx, "k")
	require.ErrorIs(t, err, context.Canceled)

	close(o.gate)
	<-opened
}

func TestArchiveCache_EvictsUnusedArchiveAndClosesZip(t *testing.T) {
	store := storedZipFake(t, "a", 16)
	data, _ := store.GetContent("a")
	require.NoError(t, store.Put(context.Background(), "b", bytes.NewReader(data), 0, ""))
	o := &countingOpener{store: store} //nolint:exhaustruct // no gate
	c := newArchiveCache(1, o.open)

	a, err := c.acquire(t.Context(), "a")
	require.NoError(t, err)
	c.release(a)

	b, err := c.acquire(t.Context(), "b")
	require.NoError(t, err)
	assert.False(t, readable(a), "evicting an unused archive closes its zip")
	c.release(b)
}
