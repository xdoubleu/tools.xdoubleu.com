package services

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
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
// is still building. A device that hasn't been sent the current KEPUB
// (outdated) reads an older one, whose spans a PDF-sourced KEPUB's
// regeneration can renumber, so its bookmark isn't translated then.
func (s *PositionService) KoboPosition(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	loc models.KoboLocation,
	outdated bool,
) (*models.ReadingPosition, bool) {
	if loc.Type != koboSpanType {
		return nil, false
	}
	timer := time.NewTimer(s.budget)
	defer timer.Stop()
	refs := s.resolveAll(ctx, userID, []uuid.UUID{bookID})
	if outdated && refs[bookID].pdf {
		return nil, false
	}
	maps, pending := s.mapsOf(ctx, timer, refs)
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
	return s.mapsOf(ctx, timer, s.resolveAll(ctx, userID, bookIDs))
}

// mapsOf is spanMaps for already resolved books, waiting until timer fires.
func (s *PositionService) mapsOf(
	ctx context.Context,
	timer *time.Timer,
	refs map[uuid.UUID]kepubRef,
) (map[uuid.UUID]*spanMap, map[uuid.UUID]bool) {
	maps := make(map[uuid.UUID]*spanMap, len(refs))
	waiting := make(map[uuid.UUID]spanMapWait)
	for bookID, ref := range refs {
		if m, hit := s.cache.get(ref.key); hit {
			maps[bookID] = m
			continue
		}
		buildCtx := context.WithoutCancel(ctx)
		ch := s.builds.DoChan(fmt.Sprint(ref.key), func() (any, error) {
			if m, hit := s.cache.get(ref.key); hit {
				return m, nil
			}
			return s.build(buildCtx, ref), nil
		})
		waiting[bookID] = spanMapWait{key: ref.key, ch: ch}
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
