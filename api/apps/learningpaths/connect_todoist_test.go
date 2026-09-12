package learningpaths_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
	"tools.xdoubleu.com/gen/learningpaths/v1/learningpathsv1connect"
	"tools.xdoubleu.com/internal/crypto"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func setupTodoistClient(
	handler http.Handler,
) learningpathsv1connect.TodoistServiceClient {
	ts := httptest.NewServer(handler)
	return learningpathsv1connect.NewTodoistServiceClient(http.DefaultClient, ts.URL)
}

func TestGetTodoistConnectionStatus_NotConnected(t *testing.T) {
	client := setupTodoistClient(getRoutes())

	resp, err := client.GetTodoistConnectionStatus(
		newCtx(),
		connect.NewRequest(&learningpathsv1.GetTodoistConnectionStatusRequest{}),
	)
	require.NoError(t, err)
	assert.False(t, resp.Msg.Connected)
	assert.Empty(t, resp.Msg.ConnectedAt)
}

func TestConnectTodoist_ReturnsAuthorizeURL(t *testing.T) {
	client := setupTodoistClient(getRoutes())

	resp, err := client.ConnectTodoist(
		newCtx(), connect.NewRequest(&learningpathsv1.ConnectTodoistRequest{}),
	)
	require.NoError(t, err)
	assert.Contains(t, resp.Msg.AuthorizeUrl, "https://app.todoist.com/oauth/authorize")
	assert.Contains(t, resp.Msg.AuthorizeUrl, "state=")
}

func TestDisconnectTodoist_NoOpWhenNotConnected(t *testing.T) {
	client := setupTodoistClient(getRoutes())

	_, err := client.DisconnectTodoist(
		newCtx(), connect.NewRequest(&learningpathsv1.DisconnectTodoistRequest{}),
	)
	assert.NoError(t, err)
}

// TestGetTodoistConnectionStatus_Connected seeds a connection directly
// through the repository (bypassing the real OAuth exchange, which would
// need a live network call to Todoist) to exercise the "connected" branch of
// both GetTodoistConnectionStatus and DisconnectTodoist end to end.
func TestGetTodoistConnectionStatus_Connected(t *testing.T) {
	testSealer, err := crypto.New(testCfg.EncryptionKey)
	require.NoError(t, err)
	repo := repositories.New(testDB, testSealer).OAuthConnections

	//nolint:exhaustruct //other fields unused by this test
	tok := &oauth2.Token{AccessToken: "seeded-access-token"}
	require.NoError(t, repo.Upsert(
		t.Context(), userID, sharedmodels.OAuthProviderTodoist, tok,
	))
	t.Cleanup(func() {
		_ = repo.Delete(t.Context(), userID, sharedmodels.OAuthProviderTodoist)
	})

	client := setupTodoistClient(getRoutes())
	statusResp, err := client.GetTodoistConnectionStatus(
		newCtx(),
		connect.NewRequest(&learningpathsv1.GetTodoistConnectionStatusRequest{}),
	)
	require.NoError(t, err)
	assert.True(t, statusResp.Msg.Connected)
	assert.NotEmpty(t, statusResp.Msg.ConnectedAt)

	_, err = client.DisconnectTodoist(
		newCtx(), connect.NewRequest(&learningpathsv1.DisconnectTodoistRequest{}),
	)
	require.NoError(t, err)

	statusResp, err = client.GetTodoistConnectionStatus(
		newCtx(),
		connect.NewRequest(&learningpathsv1.GetTodoistConnectionStatusRequest{}),
	)
	require.NoError(t, err)
	assert.False(t, statusResp.Msg.Connected)
}

func TestSendItemToTodoist_InvalidItemID(t *testing.T) {
	client := setupTodoistClient(getRoutes())

	_, err := client.SendItemToTodoist(
		newCtx(), connect.NewRequest(&learningpathsv1.SendItemToTodoistRequest{
			ItemId: "not-a-uuid",
		}),
	)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestSendItemToTodoist_ItemNotFound(t *testing.T) {
	client := setupTodoistClient(getRoutes())

	_, err := client.SendItemToTodoist(
		newCtx(), connect.NewRequest(&learningpathsv1.SendItemToTodoistRequest{
			ItemId: uuid.NewString(),
		}),
	)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// TestSendItemToTodoist_NotConnected exercises the FailedPrecondition branch:
// a real item exists and is owned by the caller, but no Todoist connection
// exists — SendItem must never reach the network in this case.
func TestSendItemToTodoist_NotConnected(t *testing.T) {
	lpClient := setupClient(getRoutes())
	ctx := newCtx()

	createResp, err := lpClient.CreateLearningPath(
		ctx, connect.NewRequest(&learningpathsv1.CreateLearningPathRequest{
			Title: "Todoist Send Test Path",
			Modules: []*learningpathsv1.Module{
				{
					Title: "Week 1",
					Items: []*learningpathsv1.Item{{Description: "Send me"}},
				},
			},
		}),
	)
	require.NoError(t, err)
	itemID := createResp.Msg.LearningPath.Modules[0].Items[0].Id

	client := setupTodoistClient(getRoutes())
	_, err = client.SendItemToTodoist(
		ctx,
		connect.NewRequest(&learningpathsv1.SendItemToTodoistRequest{ItemId: itemID}),
	)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}
