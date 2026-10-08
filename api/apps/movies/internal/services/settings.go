package services

import (
	"context"
	"math"
	"slices"
	"sync"
	"time"

	"tools.xdoubleu.com/apps/movies/pkg/tmdb"
)

// providersTTL is how long the Belgian provider list is reused; it changes
// rarely and costs two TMDB requests.
//
//nolint:gochecknoglobals // overridable in tests
var providersTTL = 24 * time.Hour

// SetProvidersTTL overrides how long the provider list is cached. Tests only.
func SetProvidersTTL(d time.Duration) { providersTTL = d }

type providerCache struct {
	mu      sync.Mutex
	expires time.Time
	list    []tmdb.RegionProvider
}

// Settings returns the TMDB provider IDs of the user's streaming services.
func (s *MovieService) Settings(ctx context.Context, userID string) ([]int64, error) {
	return s.repo.GetProviderIDs(ctx, userID)
}

// SetSettings saves the user's streaming services, sorted and without
// duplicates, and returns them.
func (s *MovieService) SetSettings(
	ctx context.Context,
	userID string,
	ids []int64,
) ([]int64, error) {
	for _, id := range ids {
		if id <= 0 || id > math.MaxInt32 {
			return nil, badRequest("invalid provider id")
		}
	}
	ids = slices.Clone(ids)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if ids == nil {
		ids = []int64{}
	}
	return ids, s.repo.SetProviderIDs(ctx, userID, ids)
}

// AvailableProviders lists the Belgian providers to pick services from,
// cached for a day. A failed refresh serves the stale list when there is one.
func (s *MovieService) AvailableProviders(
	ctx context.Context,
) ([]tmdb.RegionProvider, error) {
	if s.tmdb == nil {
		return nil, ErrNotConfigured
	}
	c := &s.providers
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.list != nil && time.Now().Before(c.expires) {
		return c.list, nil
	}
	list, err := s.tmdb.ListRegionProviders(ctx)
	if err != nil {
		if c.list != nil {
			return c.list, nil
		}
		return nil, err
	}
	c.list, c.expires = list, time.Now().Add(providersTTL)
	return list, nil
}
