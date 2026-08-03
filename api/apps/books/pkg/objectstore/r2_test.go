package objectstore_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/pkg/objectstore"
)

// newTestR2 creates an R2 client pointed at a fake endpoint with stub creds.
// No real network calls are made in unit tests; only presign URL construction
// is verified (which is purely local computation).
func newTestR2() objectstore.Client {
	return objectstore.NewR2(
		"https://test-account.r2.cloudflarestorage.com",
		"test-access-key",
		"test-secret-key",
		"test-bucket",
	)
}

func TestR2PresignGet_URLContainsKey(t *testing.T) {
	t.Parallel()

	client := newTestR2()

	// PresignGetObject is pure local computation — no HTTP call is made.
	url, err := client.PresignGet(
		context.Background(),
		"users/abc/books/xyz/file.epub",
		time.Minute*5,
	)
	require.NoError(t, err)

	assert.Contains(t, url, "test-account.r2.cloudflarestorage.com")
	assert.Contains(t, url, "test-bucket")
	assert.Contains(t, url, "users/abc/books/xyz/file.epub")
	assert.Contains(t, url, "X-Amz-Expires")
}

func TestR2PresignGet_PathStyle(t *testing.T) {
	t.Parallel()

	client := newTestR2()

	url, err := client.PresignGet(context.Background(), "some/key", time.Hour)
	require.NoError(t, err)

	// Path-style: bucket appears as a path segment, not a subdomain.
	assert.True(
		t,
		strings.Contains(url, "/test-bucket/"),
		"expected path-style URL, got: %s", url,
	)
}

func TestR2PresignGet_ExpiryEncoded(t *testing.T) {
	t.Parallel()

	client := newTestR2()

	const ttl = 5 * time.Minute
	url, err := client.PresignGet(context.Background(), "a/b/c", ttl)
	require.NoError(t, err)

	// X-Amz-Expires is in seconds.
	assert.Contains(t, url, "X-Amz-Expires=300")
}

func TestR2PresignPut_URLContainsKey(t *testing.T) {
	t.Parallel()

	client := newTestR2()

	url, err := client.PresignPut(
		context.Background(),
		"users/abc/uploads/uuid.epub",
		60*time.Minute,
		"application/epub+zip",
	)
	require.NoError(t, err)

	assert.Contains(t, url, "test-account.r2.cloudflarestorage.com")
	assert.Contains(t, url, "test-bucket")
	assert.Contains(t, url, "users/abc/uploads/uuid.epub")
	assert.Contains(t, url, "X-Amz-Expires=3600")
}
