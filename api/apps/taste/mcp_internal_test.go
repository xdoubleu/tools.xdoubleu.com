package taste

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/internal/consumption"
	tastev1 "tools.xdoubleu.com/gen/taste/v1"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/logging"
	"tools.xdoubleu.com/internal/kit/testhelper"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/family"
	"tools.xdoubleu.com/internal/platform/mcptools"
)

type staticSource []consumption.Event

func (staticSource) GetName() string { return "taste" }

func (s staticSource) ListConsumption(
	context.Context, string,
) ([]consumption.Event, error) {
	return s, nil
}

func TestMCPTools(t *testing.T) {
	const uid = "mcp-taste-user"
	cfg := testhelper.NewTestConfig()
	db := testhelper.ConnectTestDB(cfg.DBDsn)
	a := New(
		context.Background(),
		sharedmocks.NewMockedAuthService(uid), logging.NewNopLogger(), cfg, db,
		family.NewFamilyRepository(db), nil,
		Catalogs{
			Books: nil, Movies: nil, Games: nil, Podcasts: nil, Feeds: nil,
			WantBook: nil, WantTitle: nil,
		},
		staticSource{
			//nolint:exhaustruct // only what the tools return
			{Domain: consumption.DomainPodcasts, Title: "Show", At: time.Now(), Signal: 2},
		},
	)
	//nolint:exhaustruct // a name is all the server needs
	a.RegisterMCPTools(mcp.NewServer(&mcp.Implementation{Name: "test"}, nil))
	h := &tasteConnectHandler{app: a}
	ctx := context.WithValue(
		context.Background(),
		constants.UserContextKey,
		//nolint:exhaustruct // only ID and AppAccess matter for the gates
		sharedmodels.User{ID: uid, AppAccess: []string{mcpAppName}},
	)

	msg, err := h.mcpGetStats(ctx, mcpSharedArgs{Shared: false})
	require.NoError(t, err)
	stats, ok := msg.(*tastev1.GetStatsResponse)
	require.True(t, ok)
	assert.Equal(t, int32(1), stats.Domains[5].Consumed)

	msg, err = h.mcpListEvents(
		ctx,
		mcpListEventsArgs{Domain: "podcasts", Limit: 5, Offset: 0, Shared: false},
	)
	require.NoError(t, err)
	events, ok := msg.(*tastev1.ListEventsResponse)
	require.True(t, ok)
	require.Len(t, events.Events, 1)
	assert.Equal(t, "Show", events.Events[0].Title)

	msg, err = h.mcpListRecommendations(ctx, mcpListRecommendationsArgs{
		Domain: "podcasts", Track: "more", SavedOnly: false, Shared: false,
	})
	require.NoError(t, err)
	recs, ok := msg.(*tastev1.ListRecommendationsResponse)
	require.True(t, ok)
	assert.Empty(t, recs.Recommendations)

	msg, err = h.mcpGetProfile(ctx, mcpSharedArgs{Shared: false})
	require.NoError(t, err)
	profile, ok := msg.(*tastev1.GetProfileResponse)
	require.True(t, ok)
	assert.False(t, profile.Configured)

	_, err = h.mcpGetStats(context.Background(), mcpSharedArgs{Shared: false})
	require.Error(t, err)
}

// TestMCPFavouriteShowTools: the show tools go through the Connect handlers,
// scoped to the caller in context.
func TestMCPFavouriteShowTools(t *testing.T) {
	const uid, other = "mcp-taste-shows-user", "mcp-taste-shows-other"
	cfg := testhelper.NewTestConfig()
	db := testhelper.ConnectTestDB(cfg.DBDsn)
	a := NewInner(
		context.Background(),
		sharedmocks.NewMockedAuthService(uid), logging.NewNopLogger(), cfg, db,
		family.NewFamilyRepository(db), nil, nil,
		Catalogs{
			Books: nil, Movies: nil, Games: nil, Podcasts: nil, Feeds: nil,
			WantBook: nil, WantTitle: nil,
		},
	)
	ctx := context.Background()
	require.NoError(t, a.ApplyMigrations(ctx, db))
	//nolint:exhaustruct // a name is all the server needs
	a.RegisterMCPTools(mcp.NewServer(&mcp.Implementation{Name: "test"}, nil))
	h := &tasteConnectHandler{app: a}

	clean := func() {
		_, err := db.Exec(ctx,
			"DELETE FROM podcasts.shows WHERE user_id IN ($1, $2)", uid, other)
		require.NoError(t, err)
	}
	clean()
	t.Cleanup(clean)
	_, err := db.Exec(ctx, `
		INSERT INTO podcasts.shows (user_id, itunes_id, title)
		VALUES ($1, 42, 'Mcp Show')`, uid,
	)
	require.NoError(t, err)

	userCtx := func(id string) context.Context {
		return context.WithValue(
			context.Background(),
			constants.UserContextKey,
			//nolint:exhaustruct // only ID and AppAccess matter for the gates
			sharedmodels.User{ID: id, AppAccess: []string{mcpAppName}},
		)
	}
	msg, err := h.mcpListFavouriteShows(userCtx(uid), mcptools.NoArgs{})
	require.NoError(t, err)
	favs, ok := msg.(*tastev1.ListFavouriteShowsResponse)
	require.True(t, ok)
	require.Len(t, favs.Favourites, 1)
	assert.Equal(t, "Mcp Show", favs.Favourites[0].Title)

	// Another user sees none of it and can't remove it.
	msg, err = h.mcpListFavouriteShows(userCtx(other), mcptools.NoArgs{})
	require.NoError(t, err)
	favs, ok = msg.(*tastev1.ListFavouriteShowsResponse)
	require.True(t, ok)
	assert.Empty(t, favs.Favourites)
	_, err = h.mcpRemoveFavouriteShow(
		userCtx(other), mcpRemoveFavouriteShowArgs{ID: uuid.NewString()},
	)
	require.Error(t, err)

	_, err = h.mcpAddFavouriteShow(
		userCtx(uid), mcpAddFavouriteShowArgs{ITunesID: 0},
	)
	require.Error(t, err)
}
