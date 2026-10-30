package positions

import (
	"context"
	"errors"
	"fmt"

	"tools.xdoubleu.com/apps/books/pkg/ebooks"
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
	defer kepub.Close()
	if ref.pdf {
		return buildSpanMap(ctx, kepub.Reader(), nil)
	}
	source, err := s.openZip(ctx, ref.sourceKey)
	if err != nil {
		return nil, err
	}
	defer source.Close()
	return buildSpanMap(ctx, kepub.Reader(), source.Reader())
}

func (s *PositionService) openZip(
	ctx context.Context,
	key string,
) (*ebooks.TempZip, error) {
	z, err := ebooks.OpenStoredZip(ctx, s.objectStore, key, s.maxDownload)
	if errors.Is(err, ebooks.ErrZipDownload) {
		return nil, fmt.Errorf("%w: %w", errSpanMapRetry, err)
	}
	return z, err
}
