package objectstore_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/pkg/objectstore"
)

// newServerR2 starts a minimal S3-compatible httptest server and returns
// an R2 client pointed at it. The caller must call srv.Close().
//
// The mock server handles:
//
//	PUT  /bucket/key  → 200
//	GET  /bucket/key  → 200 + stored body (or 404)
//	HEAD /bucket/key  → 200 (or 404)
//	DELETE /bucket/key → 204
func newServerR2(t *testing.T) (*httptest.Server, objectstore.Client) {
	t.Helper()

	store := map[string]string{}

	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.URL.Path // e.g. /bucket/mykey

			switch r.Method {
			case http.MethodPut:
				body, _ := io.ReadAll(r.Body)
				store[key] = string(body)
				w.WriteHeader(http.StatusOK)

			case http.MethodGet:
				body, ok := store[key]
				if !ok {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(body))

			case http.MethodHead:
				if _, ok := store[key]; !ok {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.WriteHeader(http.StatusOK)

			case http.MethodDelete:
				delete(store, key)
				w.WriteHeader(http.StatusNoContent)

			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		}),
	)

	client := objectstore.NewR2(srv.URL, "fake-key", "fake-secret", "bucket")
	return srv, client
}

// TestR2Put_NoChecksumHeaders asserts that Put does not send the SDK's default
// CRC32 checksum headers (X-Amz-Sdk-Checksum-Algorithm / X-Amz-Checksum-Crc32).
// Cloudflare R2 rejects those headers with 403 AccessDenied.
func TestR2Put_NoChecksumHeaders(t *testing.T) {
	t.Parallel()

	var capturedHeader http.Header
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPut {
				capturedHeader = r.Header.Clone()
				_, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
		}),
	)
	defer srv.Close()

	client := objectstore.NewR2(srv.URL, "fake-key", "fake-secret", "bucket")

	r := strings.NewReader("epub content")
	err := client.Put(t.Context(), "mykey", r, int64(r.Len()), "application/epub+zip")
	require.NoError(t, err)

	assert.Empty(
		t,
		capturedHeader.Get("X-Amz-Sdk-Checksum-Algorithm"),
		"R2 rejects CRC32 checksum headers with 403",
	)
	assert.Empty(
		t,
		capturedHeader.Get("X-Amz-Checksum-Crc32"),
		"R2 rejects CRC32 checksum headers with 403",
	)
}

func TestR2Put_StoresObject(t *testing.T) {
	t.Parallel()

	srv, client := newServerR2(t)
	defer srv.Close()

	r := strings.NewReader("epub content")
	err := client.Put(t.Context(), "mykey", r, int64(r.Len()), "application/epub+zip")
	require.NoError(t, err)
}

func TestR2Get_ReturnsBody(t *testing.T) {
	t.Parallel()

	srv, client := newServerR2(t)
	defer srv.Close()

	r := strings.NewReader("book data")
	require.NoError(
		t,
		client.Put(
			t.Context(),
			"getkey",
			r,
			int64(r.Len()),
			"application/octet-stream",
		),
	)

	rc, err := client.Get(t.Context(), "getkey")
	require.NoError(t, err)
	defer rc.Close()

	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "book data", string(got))
}

func TestR2Exists_TrueAfterPut(t *testing.T) {
	t.Parallel()

	srv, client := newServerR2(t)
	defer srv.Close()

	exists, err := client.Exists(t.Context(), "exkey")
	require.NoError(t, err)
	assert.False(t, exists)

	r := strings.NewReader("data")
	require.NoError(
		t,
		client.Put(t.Context(), "exkey", r, int64(r.Len()), "text/plain"),
	)

	exists, err = client.Exists(t.Context(), "exkey")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestR2Delete_RemovesObject(t *testing.T) {
	t.Parallel()

	srv, client := newServerR2(t)
	defer srv.Close()

	r := strings.NewReader("to delete")
	require.NoError(
		t,
		client.Put(t.Context(), "delkey", r, int64(r.Len()), "text/plain"),
	)

	require.NoError(t, client.Delete(t.Context(), "delkey"))

	exists, err := client.Exists(t.Context(), "delkey")
	require.NoError(t, err)
	assert.False(t, exists)
}
