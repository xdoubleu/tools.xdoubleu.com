//nolint:exhaustruct // sparse tool arguments
package taste_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/internal/consumption"
	"tools.xdoubleu.com/apps/taste"
	tastev1 "tools.xdoubleu.com/gen/taste/v1"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/logging"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

type staticSource []consumption.Event

func (staticSource) GetName() string { return "taste" }

func (s staticSource) ListConsumption(
	context.Context, string,
) ([]consumption.Event, error) {
	return s, nil
}

type mcpUser struct {
	t       *testing.T
	session *mcp.ClientSession
}

// newMCPUser serves a taste app's tools as uid; withAccess sets whether uid may
// open the taste app.
func newMCPUser(
	t *testing.T,
	uid string,
	withAccess bool,
	sources ...taste.Source,
) mcpUser {
	t.Helper()
	ctx := context.Background()
	a := taste.NewInner(
		ctx, sharedmocks.NewMockedAuthService(uid), logging.NewNopLogger(),
		testCfg, testDB, testFamily, nil, nil,
		taste.Catalogs{
			Books: nil, Movies: nil, Games: nil, Podcasts: nil, Feeds: nil,
			WantBook: nil, WantTitle: nil,
		},
		sources...,
	)
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	a.RegisterMCPTools(srv)
	srv.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			//nolint:exhaustruct // only ID and AppAccess matter for the gates
			user := sharedmodels.User{ID: uid}
			if withAccess {
				user.AppAccess = []string{"taste"}
			}
			return next(context.WithValue(ctx, constants.UserContextKey, user), method, req)
		}
	})
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })
	c := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "0"}, nil)
	session, err := c.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return mcpUser{t: t, session: session}
}

// call runs the tool, decoding its result into out; a tool error is returned.
func (u mcpUser) call(name string, args map[string]any, out proto.Message) error {
	u.t.Helper()
	res, err := u.session.CallTool(
		u.t.Context(), &mcp.CallToolParams{Name: name, Arguments: args},
	)
	require.NoError(u.t, err, name)
	text, ok := res.Content[0].(*mcp.TextContent)
	require.True(u.t, ok)
	if res.IsError {
		return errors.New(text.Text)
	}
	return protojson.Unmarshal([]byte(text.Text), out)
}

func TestMCPTools(t *testing.T) {
	u := newMCPUser(t, "mcp-taste-user", true, staticSource{
		//nolint:exhaustruct // only what the tools return
		{Domain: consumption.DomainPodcasts, Title: "Show", At: time.Now(), Signal: 2},
	})

	var stats tastev1.GetStatsResponse
	require.NoError(t, u.call("taste_get_stats", nil, &stats))
	assert.Equal(t, int32(1), stats.Domains[5].Consumed)

	var events tastev1.ListEventsResponse
	require.NoError(t, u.call(
		"taste_list_events", map[string]any{"domain": "podcasts", "limit": 5}, &events,
	))
	require.Len(t, events.Events, 1)
	assert.Equal(t, "Show", events.Events[0].Title)

	var recs tastev1.ListRecommendationsResponse
	require.NoError(t, u.call(
		"taste_list_recommendations",
		map[string]any{"domain": "podcasts", "track": "more"}, &recs,
	))
	assert.Empty(t, recs.Recommendations)

	var profile tastev1.GetProfileResponse
	require.NoError(t, u.call("taste_get_profile", nil, &profile))
	assert.False(t, profile.Configured)
}

func TestMCPToolsNeedAppAccess(t *testing.T) {
	u := newMCPUser(t, "mcp-taste-denied", false)
	var stats tastev1.GetStatsResponse
	require.Error(t, u.call("taste_get_stats", nil, &stats))
}

// TestMCPFavouriteShowTools: the show tools are scoped to the caller.
func TestMCPFavouriteShowTools(t *testing.T) {
	const uid, other = "mcp-taste-shows-user", "mcp-taste-shows-other"
	ctx := context.Background()
	clean := func() {
		_, err := testDB.Exec(ctx,
			"DELETE FROM podcasts.shows WHERE user_id IN ($1, $2)", uid, other)
		require.NoError(t, err)
	}
	clean()
	t.Cleanup(clean)
	_, err := testDB.Exec(ctx, `
		INSERT INTO podcasts.shows (user_id, itunes_id, title)
		VALUES ($1, 42, 'Mcp Show')`, uid,
	)
	require.NoError(t, err)

	mine := newMCPUser(t, uid, true)
	var favs tastev1.ListFavouriteShowsResponse
	require.NoError(t, mine.call("taste_list_favourite_shows", nil, &favs))
	require.Len(t, favs.Favourites, 1)
	assert.Equal(t, "Mcp Show", favs.Favourites[0].Title)

	// Another user sees none of it and can't remove it.
	theirs := newMCPUser(t, other, true)
	var theirFavs tastev1.ListFavouriteShowsResponse
	require.NoError(t, theirs.call("taste_list_favourite_shows", nil, &theirFavs))
	assert.Empty(t, theirFavs.Favourites)
	var removed tastev1.RemoveFavouriteShowResponse
	require.Error(t, theirs.call(
		"taste_remove_favourite_show", map[string]any{"id": uuid.NewString()}, &removed,
	))

	var added tastev1.AddFavouriteShowResponse
	require.Error(t, mine.call(
		"taste_add_favourite_show", map[string]any{"itunes_id": 0}, &added,
	))
}
