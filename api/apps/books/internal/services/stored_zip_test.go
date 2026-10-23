//nolint:testpackage // testing unexported service helpers
package services

import (
	"archive/zip"
	"bytes"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/pkg/objectstore"
)

// storedZipFake stores a zip with one uncompressed entry of size bytes.
func storedZipFake(t *testing.T, key string, size int) *objectstore.FakeClient {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateHeader(&zip.FileHeader{ //nolint:exhaustruct // stored entry
		Name: "big.bin", Method: zip.Store,
	})
	require.NoError(t, err)
	_, err = w.Write(make([]byte, size))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	store := objectstore.NewFake()
	require.NoError(t, store.Put(
		t.Context(), key, &buf, int64(buf.Len()), "application/epub+zip",
	))
	return store
}

func TestOpenStoredZip_StreamsToDisk(t *testing.T) {
	const size = 32 << 20
	svc := &BookService{ //nolint:exhaustruct // partial
		objectStore: storedZipFake(t, "big.epub", size),
	}

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	z, err := svc.openStoredZip(t.Context(), "big.epub")
	runtime.ReadMemStats(&after)

	require.NoError(t, err)
	defer z.close()
	assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(size/8),
		"the stored zip is buffered on disk, not in memory")
}

func TestOpenTempZip_SizeCapLeavesNoTempFiles(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	store := storedZipFake(t, "k", 1024)
	data, ok := store.GetContent("k")
	require.True(t, ok)

	z, err := openTempZip(t.Context(), store, "k", int64(len(data)))
	require.NoError(t, err, "exactly the cap is allowed")
	entries, err := os.ReadDir(tmp)
	require.NoError(t, err)
	assert.Empty(t, entries, "the temp file is unlinked while open")
	z.close()

	_, err = openTempZip(t.Context(), store, "k", int64(len(data))-1)
	require.ErrorContains(t, err, "exceeds")
	require.NotErrorIs(t, err, errZipDownload)

	_, err = openTempZip(t.Context(), store, "missing", 1)
	require.ErrorIs(t, err, errZipDownload)

	entries, err = os.ReadDir(tmp)
	require.NoError(t, err)
	assert.Empty(t, entries)
}
