package movies_test

import (
	"context"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/movies/pkg/tmdb"
	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
	"tools.xdoubleu.com/gen/movies/v1/moviesv1connect"
)

func setServices(
	t *testing.T,
	c moviesv1connect.MoviesServiceClient,
	ids ...int64,
) []int64 {
	t.Helper()
	resp, err := c.SetSettings(context.Background(), connect.NewRequest(
		&moviesv1.SetSettingsRequest{ProviderIds: ids},
	))
	require.NoError(t, err)
	return resp.Msg.ProviderIds
}

func getServices(t *testing.T, c moviesv1connect.MoviesServiceClient) []int64 {
	t.Helper()
	resp, err := c.GetSettings(context.Background(), connect.NewRequest(
		&moviesv1.GetSettingsRequest{},
	))
	require.NoError(t, err)
	return resp.Msg.ProviderIds
}

func TestSettings_SaveSortsAndDeduplicates(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	other := newKeepingClient(t, otherUserID, fakeTMDB{})
	assert.Empty(t, getServices(t, c))

	assert.Equal(t, []int64{8, 119}, setServices(t, c, 119, 8, 119))
	assert.Equal(t, []int64{8, 119}, getServices(t, c))
	assert.Empty(t, getServices(t, other), "services are per user")

	assert.Empty(t, setServices(t, c))
	assert.Empty(t, getServices(t, c))
}

func TestSettings_RejectsInvalidProviderIDs(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})

	for _, id := range []int64{0, -1, 1 << 40} {
		_, err := c.SetSettings(context.Background(), connect.NewRequest(
			&moviesv1.SetSettingsRequest{ProviderIds: []int64{8, id}},
		))
		assert.Equal(t, connect.CodeInvalidArgument, code(err), id)
	}
	assert.Empty(t, getServices(t, c))
}

// countingTMDB counts provider list fetches; once broken, they fail.
type countingTMDB struct {
	fakeTMDB
	calls  *atomic.Int32
	broken *atomic.Bool
}

func (f countingTMDB) ListRegionProviders(
	ctx context.Context,
) ([]tmdb.RegionProvider, error) {
	f.calls.Add(1)
	if f.broken.Load() {
		return nil, assert.AnError
	}
	return f.fakeTMDB.ListRegionProviders(ctx)
}

func availableProviders(
	t *testing.T,
	c moviesv1connect.MoviesServiceClient,
) (*moviesv1.ListAvailableProvidersResponse, error) {
	t.Helper()
	resp, err := c.ListAvailableProviders(context.Background(), connect.NewRequest(
		&moviesv1.ListAvailableProvidersRequest{},
	))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

func TestAvailableProviders_Cached(t *testing.T) {
	counting := countingTMDB{
		fakeTMDB: fakeTMDB{}, calls: &atomic.Int32{}, broken: &atomic.Bool{},
	}
	c := newClient(t, userID, counting)

	got, err := availableProviders(t, c)
	require.NoError(t, err)
	require.Len(t, got.Providers, 2)
	assert.Equal(t, "Netflix", got.Providers[0].Name)
	assert.Equal(t, int64(8), got.Providers[0].Id)
	assert.Equal(t, "/netflix.jpg", got.Providers[0].LogoPath)

	_, err = availableProviders(t, c)
	require.NoError(t, err)
	assert.Equal(t, int32(1), counting.calls.Load(), "the list is cached")
}

func TestAvailableProviders_WithoutTMDB(t *testing.T) {
	c := newClient(t, userID, nil)

	_, err := availableProviders(t, c)

	assert.Equal(t, connect.CodeFailedPrecondition, code(err))
}

func TestListBacklog_OnMyServices(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	add(t, c, "movie", 603, "want")    // Netflix + Prime stream, Apple TV rent
	add(t, c, "movie", 129, "want")    // no providers
	add(t, c, "series", 70523, "want") // ads only
	onMine := &moviesv1.ListBacklogRequest{OnMyServices: true}

	assert.Empty(t, list(t, c, onMine), "no services picked yet")

	setServices(t, c, 8)
	assert.Equal(t, []string{"The Matrix"}, list(t, c, onMine))
	flags := map[string]bool{}
	for _, e := range backlog(t, c, &moviesv1.ListBacklogRequest{}) {
		flags[e.Title] = e.OnMyServices
	}
	assert.Equal(t, map[string]bool{
		"The Matrix": true, "Spirited Away": false, "Dark": false,
	}, flags)

	setServices(t, c, 350)
	assert.Empty(t, list(t, c, onMine), "rent doesn't count")

	setServices(t, c, 99)
	assert.Equal(t, []string{"Dark"}, list(t, c, onMine), "ads count")
}

func TestListBacklog_OnMyServicesIsPerUser(t *testing.T) {
	mine := newClient(t, userID, fakeTMDB{})
	add(t, mine, "movie", 603, "want")
	theirs := newKeepingClient(t, otherUserID, fakeTMDB{})
	add(t, theirs, "movie", 603, "want")

	setServices(t, mine, 8)

	req := &moviesv1.ListBacklogRequest{OnMyServices: true}
	assert.Equal(t, []string{"The Matrix"}, list(t, mine, req))
	assert.Empty(t, list(t, theirs, req))
}

func TestGetTitle_MarksProvidersOnMyServices(t *testing.T) {
	c := newClient(t, userID, fakeTMDB{})
	e := add(t, c, "movie", 603, "want")
	setServices(t, c, 8, 350)

	mine := map[string]bool{}
	for _, o := range getTitle(t, c, e.Id).Offers {
		for _, p := range o.Providers {
			mine[p.Name] = p.Mine
		}
	}

	assert.Equal(t, map[string]bool{
		"Netflix": true, "Prime Video": false, "Apple TV": false,
	}, mine, "only streaming offers on picked services, not rent")
}
