package ebooks

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"tools.xdoubleu.com/apps/books/pkg/objectstore"
)

// ErrZipDownload marks a failed download, as opposed to a bad or oversized
// zip.
var ErrZipDownload = errors.New("download zip")

// MaxEPUBUncompressedBytes bounds what kepubify inflates into memory;
// archive/zip refuses to read past an entry's declared size, so checking the
// declared sizes is enough to stop a zip bomb.
const MaxEPUBUncompressedBytes = 1 << 30

// CheckEPUBSize rejects an archive whose entries inflate past
// MaxEPUBUncompressedBytes.
func CheckEPUBSize(zr *zip.Reader) error {
	var total uint64
	for _, f := range zr.File {
		total += f.UncompressedSize64
		if total > MaxEPUBUncompressedBytes {
			return fmt.Errorf("epub inflates past %d bytes", MaxEPUBUncompressedBytes)
		}
	}
	return nil
}

// TempZip is a stored zip held in an already-unlinked temp file, so nothing is
// left on disk once it's closed or the process dies.
type TempZip struct {
	zr *zip.Reader
	f  *os.File
}

// Close releases the underlying temp file.
func (t *TempZip) Close() {
	_ = t.f.Close()
}

// Reader returns the zip reader over the temp file.
func (t *TempZip) Reader() *zip.Reader {
	return t.zr
}

// OpenStoredZip downloads key to a temp file rather than memory: books can be
// large and the API's memory limit is tight.
func OpenStoredZip(
	ctx context.Context,
	store objectstore.Client,
	key string,
	maxBytes int64,
) (*TempZip, error) {
	rc, err := store.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("%w %s: %w", ErrZipDownload, key, err)
	}
	defer func() { _ = rc.Close() }()

	f, err := os.CreateTemp("", "book-*.zip")
	if err != nil {
		return nil, fmt.Errorf("%w: create temp file: %w", ErrZipDownload, err)
	}
	_ = os.Remove(f.Name())

	n, err := io.Copy(f, io.LimitReader(rc, maxBytes+1))
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("%w %s: %w", ErrZipDownload, key, err)
	}
	if n > maxBytes {
		err = fmt.Errorf("exceeds %d bytes", maxBytes)
	}
	var zr *zip.Reader
	if err == nil {
		zr, err = zip.NewReader(f, n)
	}
	if err == nil {
		err = CheckEPUBSize(zr)
	}
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("open %s: %w", key, err)
	}
	return &TempZip{zr: zr, f: f}, nil
}
