package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/internal/database"
)

const (
	// translateBudget is how long a request waits for span maps before
	// falling back to percent (ADR-0017); builds carry on for the next one.
	translateBudget     = 3 * time.Second
	spanMapBuildTimeout = time.Minute
	// spanMapPendingGrace bounds how long a span map counts as pending, so a
	// book that never builds still syncs as percent only.
	spanMapPendingGrace = 5 * time.Minute
	spanMapCacheSize    = 16
	// spanMapBuildSlots caps concurrent builds; each holds two parsed books.
	spanMapBuildSlots = 2
)

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
	pendingGrace time.Duration
	maxDownload  int64
	now          func() time.Time
	pendingMu    sync.Mutex
	pendingSince map[spanMapKey]time.Time
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
		pendingGrace: spanMapPendingGrace,
		maxDownload:  maxConversionInputBytes,
		now:          time.Now,
		pendingMu:    sync.Mutex{},
		pendingSince: map[spanMapKey]time.Time{},
	}
}

// KoboPosition is the neutral position of a KoboSpan bookmark, nil when it
// can't be translated within the budget; pending is true while its span map
// is still building.
func (s *PositionService) KoboPosition(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	loc models.KoboLocation,
) (*models.ReadingPosition, bool) {
	if loc.Type != koboSpanType {
		return nil, false
	}
	maps, pending := s.spanMaps(ctx, userID, []uuid.UUID{bookID}, s.budget)
	if m := maps[bookID]; m != nil {
		return m.position(loc), false
	}
	return nil, pending[bookID]
}

// BackfillKoboPosition waits up to the build timeout for loc's span map and
// passes the translated position to store, for a write that couldn't wait.
func (s *PositionService) BackfillKoboPosition(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	loc models.KoboLocation,
	store func(context.Context, models.ReadingPosition) error,
) {
	maps, _ := s.spanMaps(ctx, userID, []uuid.UUID{bookID}, s.buildTimeout)
	m := maps[bookID]
	if m == nil {
		return
	}
	if pos := m.position(loc); pos != nil {
		if err := store(ctx, *pos); err != nil {
			s.logger.WarnContext(ctx, "kobo position backfill failed",
				"book_id", bookID, "err", err)
		}
	}
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
	maps, pending := s.spanMaps(ctx, userID, ids, s.budget)

	locs := make(map[uuid.UUID]*models.KoboLocation, len(maps))
	for id, m := range maps {
		if loc := m.location(positions[id]); loc != nil {
			locs[id] = loc
		}
	}
	return locs, pending
}

type spanMapWait struct {
	key spanMapKey
	ch  <-chan singleflight.Result
}

// spanMaps returns the span maps of the books that have one, waiting up to
// budget (lookups included) for builds. pending holds the books still
// building, or whose build failed in a way worth retrying, within the grace.
func (s *PositionService) spanMaps(
	ctx context.Context,
	userID string,
	bookIDs []uuid.UUID,
	budget time.Duration,
) (map[uuid.UUID]*spanMap, map[uuid.UUID]bool) {
	timer := time.NewTimer(budget)
	defer timer.Stop()

	maps := make(map[uuid.UUID]*spanMap, len(bookIDs))
	waiting := make(map[uuid.UUID]spanMapWait)
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
		ch := s.builds.DoChan(fmt.Sprint(key), func() (any, error) {
			if m, hit := s.cache.get(key); hit {
				return m, nil
			}
			return s.build(buildCtx, key, kepubKey, sourceKey), nil
		})
		waiting[bookID] = spanMapWait{key: key, ch: ch}
	}

	return maps, s.await(ctx, timer, waiting, maps)
}

// await collects finished builds into maps until timer fires or ctx ends,
// returning the books still pending.
func (s *PositionService) await(
	ctx context.Context,
	timer *time.Timer,
	waiting map[uuid.UUID]spanMapWait,
	maps map[uuid.UUID]*spanMap,
) map[uuid.UUID]bool {
	pending := make(map[uuid.UUID]bool)
	expired := false
	for bookID, w := range waiting {
		var res singleflight.Result
		got := false
		if !expired {
			select {
			case res = <-w.ch:
				got = true
			case <-timer.C:
				expired = true
			case <-ctx.Done():
				expired = true
			}
		}
		if !got {
			select {
			case res = <-w.ch:
				got = true
			default:
			}
		}
		if m, ok := res.Val.(*spanMap); got && ok && m != nil {
			maps[bookID] = m
		} else if s.stillPending(w.key) {
			pending[bookID] = true
		}
	}
	return pending
}

// stillPending reports whether key first went pending within the grace.
func (s *PositionService) stillPending(key spanMapKey) bool {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	now := s.now()
	since, ok := s.pendingSince[key]
	if !ok {
		s.pendingSince[key] = now
		return true
	}
	return now.Sub(since) < s.pendingGrace
}

func (s *PositionService) markBuilt(key spanMapKey, m *spanMap) {
	s.cache.add(key, m)
	s.pendingMu.Lock()
	delete(s.pendingSince, key)
	s.pendingMu.Unlock()
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
