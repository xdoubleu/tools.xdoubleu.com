package learningpaths_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
	"tools.xdoubleu.com/gen/learningpaths/v1/learningpathsv1connect"
	"tools.xdoubleu.com/internal/crypto"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/todoist"
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

// TestGetTodoistConnectionStatus_Connected seeds a connection via the
// repository to cover the connected branches of status and disconnect.
func TestGetTodoistConnectionStatus_Connected(t *testing.T) {
	testSealer, err := crypto.New(testCfg.EncryptionKey)
	require.NoError(t, err)
	repo := repositories.New(testDB, testSealer).OAuthConnections

	//nolint:exhaustruct //other fields unused by this test
	tok := &oauth2.Token{AccessToken: "seeded-access-token"}
	require.NoError(t, repo.Upsert(
		t.Context(), userID, sharedmodels.OAuthProviderTodoist, tok,
		todoist.ScopeParam(todoist.RequiredScopes()),
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
