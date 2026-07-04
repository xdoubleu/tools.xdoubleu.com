package objectstore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

type fakeObject struct {
	data        []byte
	contentType string
}

// FakeClient is an in-memory Client for use in tests.
type FakeClient struct {
	mu      sync.RWMutex
	objects map[string]fakeObject
}

// NewFake returns a new FakeClient with an empty store.
func NewFake() *FakeClient {
	//nolint:exhaustruct //mu is zero-value-ready; only objects needs initialisation
	return &FakeClient{objects: make(map[string]fakeObject)}
}

func (f *FakeClient) Put(
	_ context.Context,
	key string,
	r io.Reader,
	_ int64,
	contentType string,
) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("fake put read: %w", err)
	}
	f.mu.Lock()
	f.objects[key] = fakeObject{data: data, contentType: contentType}
	f.mu.Unlock()
	return nil
}

func (f *FakeClient) Get(_ context.Context, key string) (io.ReadCloser, error) {
	f.mu.RLock()
	obj, ok := f.objects[key]
	f.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("fake get: key %q not found", key)
	}
	return io.NopCloser(bytes.NewReader(obj.data)), nil
}

func (f *FakeClient) PresignGet(
	_ context.Context,
	key string,
	ttl time.Duration,
) (string, error) {
	f.mu.RLock()
	_, ok := f.objects[key]
	f.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("fake presign: key %q not found", key)
	}
	return fmt.Sprintf("https://fake.example.com/%s?ttl=%s", key, ttl), nil
}

// PresignPut returns a fake presigned PUT URL. Unlike PresignGet, the key does
// not need to exist yet — callers simulate the client PUT via Put().
func (f *FakeClient) PresignPut(
	_ context.Context,
	key string,
	ttl time.Duration,
	_ string,
) (string, error) {
	return fmt.Sprintf("https://fake.example.com/%s?method=PUT&ttl=%s", key, ttl), nil
}

func (f *FakeClient) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	delete(f.objects, key)
	f.mu.Unlock()
	return nil
}

func (f *FakeClient) Exists(_ context.Context, key string) (bool, error) {
	f.mu.RLock()
	_, ok := f.objects[key]
	f.mu.RUnlock()
	return ok, nil
}

func (f *FakeClient) Copy(_ context.Context, srcKey, dstKey string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	obj, ok := f.objects[srcKey]
	if !ok {
		return fmt.Errorf("fake copy: src key %q not found", srcKey)
	}
	f.objects[dstKey] = fakeObject{data: obj.data, contentType: obj.contentType}
	return nil
}

// GetContent returns the raw bytes stored at key, for test assertions.
func (f *FakeClient) GetContent(key string) ([]byte, bool) {
	f.mu.RLock()
	obj, ok := f.objects[key]
	f.mu.RUnlock()
	return obj.data, ok
}
