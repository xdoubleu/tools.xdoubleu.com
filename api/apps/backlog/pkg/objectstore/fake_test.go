package objectstore_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/backlog/pkg/objectstore"
)

func TestFake_PutAndGet(t *testing.T) {
	t.Parallel()

	f := objectstore.NewFake()
	ctx := context.Background()

	body := strings.NewReader("hello epub")
	err := f.Put(ctx, "books/a.epub", body, int64(body.Len()), "application/epub+zip")
	require.NoError(t, err)

	rc, err := f.Get(ctx, "books/a.epub")
	require.NoError(t, err)
	defer rc.Close()

	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "hello epub", string(got))
}

func TestFake_GetMissing(t *testing.T) {
	t.Parallel()

	f := objectstore.NewFake()
	_, err := f.Get(context.Background(), "missing/key")
	assert.Error(t, err)
}

func TestFake_Exists(t *testing.T) {
	t.Parallel()

	f := objectstore.NewFake()
	ctx := context.Background()

	exists, err := f.Exists(ctx, "k")
	require.NoError(t, err)
	assert.False(t, exists)

	r := strings.NewReader("data")
	require.NoError(t, f.Put(ctx, "k", r, int64(r.Len()), "application/octet-stream"))

	exists, err = f.Exists(ctx, "k")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestFake_Delete(t *testing.T) {
	t.Parallel()

	f := objectstore.NewFake()
	ctx := context.Background()

	r := strings.NewReader("content")
	require.NoError(t, f.Put(ctx, "del/me", r, int64(r.Len()), "text/plain"))

	require.NoError(t, f.Delete(ctx, "del/me"))

	exists, err := f.Exists(ctx, "del/me")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestFake_PresignGet(t *testing.T) {
	t.Parallel()

	f := objectstore.NewFake()
	ctx := context.Background()

	r := strings.NewReader("epub bytes")
	require.NoError(
		t,
		f.Put(
			ctx,
			"users/u1/books/b1/f.epub",
			r,
			int64(r.Len()),
			"application/epub+zip",
		),
	)

	url, err := f.PresignGet(ctx, "users/u1/books/b1/f.epub", 5*time.Minute)
	require.NoError(t, err)
	assert.Contains(t, url, "users/u1/books/b1/f.epub")
	assert.Contains(t, url, "ttl=")
}

func TestFake_PresignGet_Missing(t *testing.T) {
	t.Parallel()

	f := objectstore.NewFake()
	_, err := f.PresignGet(context.Background(), "no/such/key", time.Minute)
	assert.Error(t, err)
}

func TestFake_PresignPut(t *testing.T) {
	t.Parallel()

	f := objectstore.NewFake()
	ctx := context.Background()

	// Key does not need to exist yet — PresignPut is called before the upload.
	url, err := f.PresignPut(
		ctx,
		"users/u1/uploads/uuid.epub",
		60*time.Minute,
		"application/epub+zip",
	)
	require.NoError(t, err)
	assert.Contains(t, url, "users/u1/uploads/uuid.epub")
	assert.Contains(t, url, "PUT")
}

func TestFake_Copy(t *testing.T) {
	t.Parallel()

	f := objectstore.NewFake()
	ctx := context.Background()

	r := strings.NewReader("epub content")
	require.NoError(
		t,
		f.Put(ctx, "src/file.epub", r, int64(r.Len()), "application/epub+zip"),
	)

	// Copy to a new key.
	require.NoError(t, f.Copy(ctx, "src/file.epub", "books/sha256abc.epub"))

	// Both keys exist with the same content.
	srcData, ok := f.GetContent("src/file.epub")
	require.True(t, ok)
	dstData, ok := f.GetContent("books/sha256abc.epub")
	require.True(t, ok)
	assert.Equal(t, srcData, dstData)
}

func TestFake_Copy_OverwritesExisting(t *testing.T) {
	t.Parallel()

	f := objectstore.NewFake()
	ctx := context.Background()

	r1 := strings.NewReader("original")
	require.NoError(t, f.Put(ctx, "src", r1, int64(r1.Len()), "text/plain"))
	r2 := strings.NewReader("old value")
	require.NoError(t, f.Put(ctx, "dst", r2, int64(r2.Len()), "text/plain"))

	// Copy src → dst should silently overwrite.
	require.NoError(t, f.Copy(ctx, "src", "dst"))

	data, ok := f.GetContent("dst")
	require.True(t, ok)
	assert.Equal(t, "original", string(data))
}

func TestFake_Copy_MissingSource(t *testing.T) {
	t.Parallel()

	f := objectstore.NewFake()
	err := f.Copy(context.Background(), "nonexistent", "dst")
	assert.Error(t, err)
}

func TestFake_GetContent(t *testing.T) {
	t.Parallel()

	f := objectstore.NewFake()
	ctx := context.Background()

	r := strings.NewReader("raw bytes")
	require.NoError(t, f.Put(ctx, "key", r, int64(r.Len()), "application/octet-stream"))

	data, ok := f.GetContent("key")
	assert.True(t, ok)
	assert.Equal(t, "raw bytes", string(data))

	_, ok = f.GetContent("nonexistent")
	assert.False(t, ok)
}
