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
	"tools.xdoubleu.com/internal/mcptools"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/testhelper"
)

type staticSource []consumption.Event

func (s staticSource) ListConsumption(
	context.Context, string,
) ([]consumption.Event, error) {
	return s, nil
}

func TestMCPTools(t *testing.T) {
	const uid = "mcp-taste-user"
	a := New(
		sharedmocks.NewMockedAuthService(uid), logging.NewNopLogger(),
		testhelper.NewTestConfig(),
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
		//nolint:exhaustruct // only ID and AppAccess matter for the gate
		sharedmodels.User{ID: uid, AppAccess: []string{mcpAppName}},
	)

	msg, err := h.mcpGetStats(ctx, mcptools.NoArgs{})
	require.NoError(t, err)
	stats, ok := msg.(*tastev1.GetStatsResponse)
	require.True(t, ok)
	assert.Equal(t, int32(1), stats.Domains[5].Consumed)

	msg, err = h.mcpListEvents(
		ctx,
		mcpListEventsArgs{Domain: "podcasts", Limit: 5, Offset: 0},
	)
	require.NoError(t, err)
	events, ok := msg.(*tastev1.ListEventsResponse)
	require.True(t, ok)
	require.Len(t, events.Events, 1)
	assert.Equal(t, "Show", events.Events[0].Title)

	_, err = h.mcpGetStats(context.Background(), mcptools.NoArgs{})
	require.Error(t, err)
}
