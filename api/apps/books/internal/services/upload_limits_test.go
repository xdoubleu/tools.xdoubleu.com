//nolint:testpackage // testing unexported service helpers
package services

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/pkg/objectstore"
)

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

// endlessStore serves an object larger than any limit; other Client methods
// are unused.
type endlessStore struct {
	objectstore.Client
	deleted string
}

func (s *endlessStore) Get(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(io.LimitReader(zeroReader{}, MaxUploadBytes+10)), nil
}

func (s *endlessStore) Delete(_ context.Context, key string) error {
	s.deleted = key
	return nil
}

// The presigned PUT doesn't bind the size, so finalize enforces it.
func TestLoadUploadedFile_OversizedRejectedAndDeleted(t *testing.T) {
	store := &endlessStore{}                //nolint:exhaustruct // Client unused
	svc := &BookService{objectStore: store} //nolint:exhaustruct // partial

	_, err := svc.loadUploadedFile(t.Context(), "users/u/uploads/x.epub")
	require.ErrorIs(t, err, ErrFileTooLarge)
	assert.Equal(t, "users/u/uploads/x.epub", store.deleted)
}

func smallEPUB(t *testing.T) *zip.Reader {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("chapter.xhtml")
	require.NoError(t, err)
	_, err = w.Write([]byte("<html/>"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	return zr
}

func TestCheckEPUBSize(t *testing.T) {
	zr := smallEPUB(t)
	require.NoError(t, checkEPUBSize(zr))

	// A zip bomb declares a huge inflated size for a tiny entry.
	zr.File[0].UncompressedSize64 = maxEPUBUncompressedBytes + 1
	require.Error(t, checkEPUBSize(zr))
}

func TestKepubifyConvert_RejectsZipBomb(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	// Stored entry with a declared size past the limit.
	//nolint:exhaustruct // only the size fields matter
	w, err := zw.CreateRaw(&zip.FileHeader{
		Name:               "bomb.xhtml",
		Method:             zip.Store,
		CompressedSize64:   1,
		UncompressedSize64: maxEPUBUncompressedBytes + 1,
	})
	require.NoError(t, err)
	_, err = w.Write([]byte("x"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	_, err = newKepubifyConverter().Convert(t.Context(), buf.Bytes())
	require.Error(t, err)
}
