package services

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/internal/database"
)

const (
	// translateBudget is how long a request waits for a span map before
	// falling back to percent (ADR-0017); the build carries on for the next.
	translateBudget     = 3 * time.Second
	spanMapBuildTimeout = time.Minute
	spanMapCacheSize    = 16
	// spanMapBuildSlots caps concurrent builds; each holds two parsed books.
	spanMapBuildSlots = 2
)

// errSpanMapRetry marks a build failure worth retrying (download, timeout)
// rather than caching.
var errSpanMapRetry = errors.New("span map build can be retried")

// kepubFileStore is the slice of BookFilesRepository the translator reads.
type kepubFileStore interface {
	GetByBookAndFormat(
		ctx context.Context, userID string, bookID uuid.UUID, format string,
	) (*models.BookFile, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.BookFile, error)
}

// PositionService translates between Kobo span bookmarks and neutral
// positions in the original EPUB (ADR-0029).
type PositionService struct {
	logger       *slog.Logger
	files        kepubFileStore
	objectStore  objectstore.Client
	cache        *spanMapCache
	builds       singleflight.Group
	slots        chan struct{}
	budget       time.Duration
	buildTimeout time.Duration
}

func newPositionService(
	logger *slog.Logger,
	files kepubFileStore,
	objectStore objectstore.Client,
	budget time.Duration,
) *PositionService {
	return &PositionService{
		logger:       logger,
		files:        files,
		objectStore:  objectStore,
		cache:        newSpanMapCache(spanMapCacheSize),
		builds:       singleflight.Group{},
		slots:        make(chan struct{}, spanMapBuildSlots),
		budget:       budget,
		buildTimeout: spanMapBuildTimeout,
	}
}

// KoboPosition is the neutral position of a KoboSpan bookmark, nil when it
// can't be translated within the budget.
func (s *PositionService) KoboPosition(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	loc models.KoboLocation,
) *models.ReadingPosition {
	if loc.Type != koboSpanType {
		return nil
	}
	maps, _ := s.spanMaps(ctx, userID, []uuid.UUID{bookID})
	if m := maps[bookID]; m != nil {
		return m.position(loc)
	}
	return nil
}

// KoboLocations translates neutral EPUB positions to KoboSpan locations.
// pending holds the books whose span map was still building at the budget.
func (s *PositionService) KoboLocations(
	ctx context.Context,
	userID string,
	positions map[uuid.UUID]models.ReadingPosition,
) (map[uuid.UUID]*models.KoboLocation, map[uuid.UUID]bool) {
	ids := make([]uuid.UUID, 0, len(positions))
	for id, pos := range positions {
		if pos.Href != "" {
			ids = append(ids, id)
		}
	}
	maps, pending := s.spanMaps(ctx, userID, ids)

	locs := make(map[uuid.UUID]*models.KoboLocation, len(maps))
	for id, m := range maps {
		if loc := m.location(positions[id]); loc != nil {
			locs[id] = loc
		}
	}
	return locs, pending
}

// spanMaps returns the span maps of the books that have one, waiting up to
// the budget for builds; pending holds the books still building or whose
// build failed in a way worth retrying.
func (s *PositionService) spanMaps(
	ctx context.Context,
	userID string,
	bookIDs []uuid.UUID,
) (map[uuid.UUID]*spanMap, map[uuid.UUID]bool) {
	maps := make(map[uuid.UUID]*spanMap, len(bookIDs))
	waiting := make(map[uuid.UUID]<-chan singleflight.Result)
	for _, bookID := range bookIDs {
		key, kepubKey, sourceKey, ok := s.resolve(ctx, userID, bookID)
		if !ok {
			continue
		}
		if m, hit := s.cache.get(key); hit {
			maps[bookID] = m
			continue
		}
		buildCtx := context.WithoutCancel(ctx)
		waiting[bookID] = s.builds.DoChan(fmt.Sprint(key), func() (any, error) {
			return s.build(buildCtx, key, kepubKey, sourceKey), nil
		})
	}

	pending := make(map[uuid.UUID]bool)
	timer := time.NewTimer(s.budget)
	defer timer.Stop()
	expired := false
	for bookID, ch := range waiting {
		if !expired {
			select {
			case res := <-ch:
				addBuilt(maps, pending, bookID, res)
				continue
			case <-timer.C:
				expired = true
			case <-ctx.Done():
				expired = true
			}
		}
		select {
		case res := <-ch:
			addBuilt(maps, pending, bookID, res)
		default:
			pending[bookID] = true
		}
	}
	return maps, pending
}

// addBuilt records a finished build; a nil map is a retryable failure.
func addBuilt(
	maps map[uuid.UUID]*spanMap,
	pending map[uuid.UUID]bool,
	bookID uuid.UUID,
	res singleflight.Result,
) {
	if m, ok := res.Val.(*spanMap); ok && m != nil {
		maps[bookID] = m
		return
	}
	pending[bookID] = true
}

// resolve finds the user's ready KEPUB of bookID and its EPUB source; ok is
// false when there's nothing to translate against.
func (s *PositionService) resolve(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (spanMapKey, string, string, bool) {
	kepub, err := s.files.GetByBookAndFormat(
		ctx, userID, bookID, models.FileFormatKEPUB,
	)
	if err == nil && kepub.Status == models.FileStatusReady &&
		kepub.SourceFileID != nil {
		var source *models.BookFile
		source, err = s.files.GetByID(ctx, *kepub.SourceFileID)
		if err == nil && source.Format == models.FileFormatEPUB {
			return spanMapKey{
				kepubID: kepub.ID, sourceID: source.ID, version: kepub.ConverterVersion,
			}, kepub.StorageKey, source.StorageKey, true
		}
	}
	if err != nil && !errors.Is(err, database.ErrResourceNotFound) {
		s.logger.WarnContext(ctx, "kepub span map lookup failed",
			"book_id", bookID, "err", err)
	}
	return spanMapKey{kepubID: uuid.Nil, sourceID: uuid.Nil, version: 0}, "", "", false
}

// build parses both files into a span map and caches it. A broken book caches
// an empty map so it isn't downloaded again on every sync; a retryable
// failure returns nil and caches nothing.
func (s *PositionService) build(
	ctx context.Context,
	key spanMapKey,
	kepubKey, sourceKey string,
) *spanMap {
	ctx, cancel := context.WithTimeout(ctx, s.buildTimeout)
	defer cancel()
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return nil
	}

	m := &spanMap{docs: nil}
	func() {
		defer func() {
			if r := recover(); r != nil {
				s.logger.ErrorContext(ctx, "kepub span map build panicked",
					"kepub_file_id", key.kepubID, "panic", r)
			}
		}()
		built, err := s.buildFromStore(ctx, kepubKey, sourceKey)
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
		s.cache.add(key, m)
	}
	return m
}

func (s *PositionService) buildFromStore(
	ctx context.Context,
	kepubKey, sourceKey string,
) (*spanMap, error) {
	kepub, err := s.openZip(ctx, kepubKey)
	if err != nil {
		return nil, err
	}
	defer kepub.close()
	source, err := s.openZip(ctx, sourceKey)
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
	n, err := io.Copy(tmp, io.LimitReader(rc, maxConversionInputBytes+1))
	_ = tmp.Close()
	if err != nil {
		_ = os.Remove(tmp.Name())
		return nil, fmt.Errorf("%w: download %s: %w", errSpanMapRetry, key, err)
	}
	if n > maxConversionInputBytes {
		err = fmt.Errorf("%s exceeds %d bytes", key, maxConversionInputBytes)
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
