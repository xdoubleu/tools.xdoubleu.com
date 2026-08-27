package feeds_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/feeds"
	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
	"tools.xdoubleu.com/gen/feeds/v1/feedsv1connect"
	"tools.xdoubleu.com/internal/mailer"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/notifications"
	"tools.xdoubleu.com/internal/testhelper"
)

// newAdminFeedsClient returns a Connect client whose app authenticates all
// requests as an admin user (RoleAdmin) — GetUnhealthyFeeds reports every
// user's feeds, so it is admin-only rather than scoped by feedUser.
func newAdminFeedsClient(t *testing.T) feedsv1connect.FeedServiceClient {
	t.Helper()
	adminApp := feeds.NewInner(
		sharedmocks.NewMockedAdminAuthService(userID),
		testApp.Logger,
		testApp.Config,
		testDB,
		mockWebFetch,
		notifications.New(
			context.Background(),
			testApp.Logger,
			mailer.New("", "", ""),
		),
		appUsersRepo,
	)
	ts := httptest.NewServer(testhelper.BuildMux(adminApp))
	t.Cleanup(ts.Close)
	return feedsv1connect.NewFeedServiceClient(http.DefaultClient, ts.URL)
}

func TestGetUnhealthyFeeds_NonAdmin_PermissionDenied(t *testing.T) {
	client := newFeedsClient(t)

	_, err := client.GetUnhealthyFeeds(
		context.Background(),
		connect.NewRequest(&feedsv1.GetUnhealthyFeedsRequest{}),
	)
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	assert.Equal(t, connect.CodePermissionDenied, connErr.Code())
}

func TestGetUnhealthyFeeds_Admin_Success(t *testing.T) {
	client := newAdminFeedsClient(t)

	resp, err := client.GetUnhealthyFeeds(
		context.Background(),
		connect.NewRequest(&feedsv1.GetUnhealthyFeedsRequest{}),
	)
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg)
}
