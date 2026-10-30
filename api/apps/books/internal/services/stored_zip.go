package services

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"tools.xdoubleu.com/apps/books/internal/services/conversion"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
)

// errZipDownload marks a failed download, as opposed to a bad or oversized
// zip.
var errZipDownload = errors.New("download zip")

// tempZip is a stored zip held in an already-unlinked temp file, so nothing is
// left on disk once it's closed or the process dies.
type tempZip struct {
	zr *zip.Reader
	f  *os.File
}

func (t *tempZip) close() {
	_ = t.f.Close()
}

// openTempZip downloads key to a temp file rather than memory: books can be
// large and the API's memory limit is tight.
func openTempZip(
	ctx context.Context,
	store objectstore.Client,
	key string,
	maxBytes int64,
) (*tempZip, error) {
	rc, err := store.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("%w %s: %w", errZipDownload, key, err)
	}
	defer func() { _ = rc.Close() }()

	f, err := os.CreateTemp("", "book-*.zip")
	if err != nil {
		return nil, fmt.Errorf("%w: create temp file: %w", errZipDownload, err)
	}
	_ = os.Remove(f.Name())

	n, err := io.Copy(f, io.LimitReader(rc, maxBytes+1))
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("%w %s: %w", errZipDownload, key, err)
	}
	if n > maxBytes {
		err = fmt.Errorf("exceeds %d bytes", maxBytes)
	}
	var zr *zip.Reader
	if err == nil {
		zr, err = zip.NewReader(f, n)
	}
	if err == nil {
		err = conversion.CheckEPUBSize(zr)
	}
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("open %s: %w", key, err)
	}
	return &tempZip{zr: zr, f: f}, nil
}
