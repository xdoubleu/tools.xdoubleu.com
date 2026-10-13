package taste

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tastev1 "tools.xdoubleu.com/gen/taste/v1"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/consumption"
	"tools.xdoubleu.com/internal/logging"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/testhelper"
)

type staticSource []consumption.Event

func (staticSource) GetName() string { return "podcasts" }

func (s staticSource) ListConsumption(
	context.Context, string,
) ([]consumption.Event, error) {
	return s, nil
}

func TestMCPTools(t *testing.T) {
	const uid = "mcp-taste-user"
	cfg := testhelper.NewTestConfig()
	a := New(
		sharedmocks.NewMockedAuthService(uid), logging.NewNopLogger(), cfg,
		testhelper.ConnectTestDB(cfg.DBDsn), nil,
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
		sharedmodels.User{ID: uid, AppAccess: []string{mcpAppName, "podcasts"}},
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
