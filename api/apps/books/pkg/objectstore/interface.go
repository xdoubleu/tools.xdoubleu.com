package objectstore

import (
	"context"
	"io"
	"time"
)

// ObjectInfo describes a single object returned by List.
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
}

type Client interface {
	Put(
		ctx context.Context,
		key string,
		r io.Reader,
		size int64,
		contentType string,
	) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	PresignPut(
		ctx context.Context,
		key string,
		ttl time.Duration,
		contentType string,
	) (string, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	// Copy duplicates the object at srcKey to dstKey within the same bucket.
	// If dstKey already exists it is silently overwritten (idempotent).
	Copy(ctx context.Context, srcKey, dstKey string) error
	// List returns every object whose key starts with prefix (pass "" for the
	// whole bucket), following pagination to completion.
	List(ctx context.Context, prefix string) ([]ObjectInfo, error)
}
