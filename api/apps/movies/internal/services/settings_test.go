package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/movies/internal/repositories"
	"tools.xdoubleu.com/apps/movies/internal/services"
	"tools.xdoubleu.com/apps/movies/pkg/tmdb"
)

type stubProviders struct {
	tmdb.Client
	list  []tmdb.RegionProvider
	err   error
	calls int
}

func (s *stubProviders) ListRegionProviders(
	context.Context,
) ([]tmdb.RegionProvider, error) {
	s.calls++
	return s.list, s.err
}

func newService(stub *stubProviders, ttl time.Duration) *services.MovieService {
	services.SetProvidersTTL(ttl)
	return services.New(&repositories.Repositories{Movies: nil}, nil, stub).Movies
}

func TestAvailableProviders_CachedWithinTTL(t *testing.T) {
	stub := &stubProviders{
		Client: nil,
		list:   []tmdb.RegionProvider{{ID: 8, Name: "Netflix", LogoPath: ""}},
		err:    nil,
		calls:  0,
	}
	s := newService(stub, time.Hour)

	for range 2 {
		_, err := s.AvailableProviders(context.Background())
		require.NoError(t, err)
	}

	assert.Equal(t, 1, stub.calls)
}

func TestAvailableProviders_RefreshesAfterTTL(t *testing.T) {
	stub := &stubProviders{
		Client: nil,
		list:   []tmdb.RegionProvider{{ID: 8, Name: "Netflix", LogoPath: ""}},
		err:    nil,
		calls:  0,
	}
	s := newService(stub, 0)
	_, err := s.AvailableProviders(context.Background())
	require.NoError(t, err)

	stub.list = []tmdb.RegionProvider{{ID: 119, Name: "Prime Video", LogoPath: ""}}
	got, err := s.AvailableProviders(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 2, stub.calls)
	assert.Equal(t, int64(119), got[0].ID)
}

func TestAvailableProviders_ServesStaleListWhenRefreshFails(t *testing.T) {
	stub := &stubProviders{
		Client: nil,
		list:   []tmdb.RegionProvider{{ID: 8, Name: "Netflix", LogoPath: ""}},
		err:    nil,
		calls:  0,
	}
	s := newService(stub, 0)
	_, err := s.AvailableProviders(context.Background())
	require.NoError(t, err)

	stub.err = errors.New("tmdb down")
	got, err := s.AvailableProviders(context.Background())

	require.NoError(t, err)
	assert.Equal(t, int64(8), got[0].ID)
}

func TestAvailableProviders_FailsWithoutAnyList(t *testing.T) {
	stub := &stubProviders{Client: nil, list: nil, err: errors.New("tmdb down"), calls: 0}
	s := newService(stub, time.Hour)

	_, err := s.AvailableProviders(context.Background())

	require.ErrorContains(t, err, "tmdb down")
}
