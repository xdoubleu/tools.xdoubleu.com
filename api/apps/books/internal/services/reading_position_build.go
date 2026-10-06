package services

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

// errSpanMapRetry marks a build failure worth retrying (download, timeout)
// rather than caching.
var errSpanMapRetry = errors.New("span map build can be retried")

// build parses both files into a span map and caches it. A broken book caches
// an empty map so it isn't downloaded again on every sync; a retryable
// failure, including no free build slot within the timeout, returns nil and
// caches nothing.
func (s *PositionService) build(ctx context.Context, ref kepubRef) *spanMap {
	key := ref.key
	waitCtx, cancelWait := context.WithTimeout(ctx, s.buildTimeout)
	defer cancelWait()
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-waitCtx.Done():
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, s.buildTimeout)
	defer cancel()

	m := &spanMap{docs: nil, pages: nil, pdf: false}
	func() {
		defer func() {
			if r := recover(); r != nil {
				s.logger.ErrorContext(ctx, "kepub span map build panicked",
					"kepub_file_id", key.kepubID, "panic", r)
			}
		}()
		built, err := s.buildFromStore(ctx, ref)
		if err != nil {
			s.logger.WarnContext(ctx, "kepub span map build failed",
				"kepub_file_id", key.kepubID, "err", err)
			if errors.Is(err, errSpanMapRetry) || ctx.Err() != nil {
				m = nil
			}
			return
		}
		m = built
	}()
	if m != nil {
		s.markBuilt(key, m)
	}
	return m
}

func (s *PositionService) buildFromStore(
	ctx context.Context,
	ref kepubRef,
) (*spanMap, error) {
	kepub, err := s.openZip(ctx, ref.kepubKey)
	if err != nil {
		return nil, err
	}
	defer kepub.close()
	if ref.pdf {
		return buildSpanMap(ctx, &kepub.zr.Reader, nil)
	}
	source, err := s.openZip(ctx, ref.sourceKey)
	if err != nil {
		return nil, err
	}
	defer source.close()
	return buildSpanMap(ctx, &kepub.zr.Reader, &source.zr.Reader)
}

type tempZip struct {
	zr   *zip.ReadCloser
	path string
}

func (t *tempZip) close() {
	_ = t.zr.Close()
	_ = os.Remove(t.path)
}

// openZip downloads key to a temp file rather than memory: books can be large
// and the API's memory limit is tight.
func (s *PositionService) openZip(ctx context.Context, key string) (*tempZip, error) {
	rc, err := s.objectStore.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("%w: download %s: %w", errSpanMapRetry, key, err)
	}
	defer func() { _ = rc.Close() }()

	tmp, err := os.CreateTemp("", "spanmap-*.zip")
	if err != nil {
		return nil, fmt.Errorf("%w: create temp file: %w", errSpanMapRetry, err)
	}
	n, err := io.Copy(tmp, io.LimitReader(rc, s.maxDownload+1))
	_ = tmp.Close()
	if err != nil {
		_ = os.Remove(tmp.Name())
		return nil, fmt.Errorf("%w: download %s: %w", errSpanMapRetry, key, err)
	}
	if n > s.maxDownload {
		err = fmt.Errorf("%s exceeds %d bytes", key, s.maxDownload)
	}
	var zr *zip.ReadCloser
	if err == nil {
		zr, err = zip.OpenReader(tmp.Name())
	}
	if err == nil {
		if err = checkEPUBSize(&zr.Reader); err != nil {
			_ = zr.Close()
		}
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return nil, fmt.Errorf("open %s: %w", key, err)
	}
	return &tempZip{zr: zr, path: tmp.Name()}, nil
}
