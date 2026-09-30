package learningpaths_test

import (
	"net/url"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
	"tools.xdoubleu.com/internal/crypto"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func seedTodoistConnection(t *testing.T, requestedScope string) {
	t.Helper()
	testSealer, err := crypto.New(testCfg.EncryptionKey)
	require.NoError(t, err)
	repo := repositories.New(testDB, testSealer).OAuthConnections

	//nolint:exhaustruct //other fields unused by this test
	tok := &oauth2.Token{AccessToken: "seeded-access-token"}
	require.NoError(t, repo.Upsert(
		t.Context(), userID, sharedmodels.OAuthProviderTodoist, tok, requestedScope,
	))
	t.Cleanup(func() {
		_ = repo.Delete(t.Context(), userID, sharedmodels.OAuthProviderTodoist)
	})
}

func TestConnectTodoist_RequestsCommaSeparatedScopes(t *testing.T) {
	client := setupTodoistClient(getRoutes())

	resp, err := client.ConnectTodoist(
		newCtx(), connect.NewRequest(&learningpathsv1.ConnectTodoistRequest{}),
	)
	require.NoError(t, err)
	parsed, err := url.Parse(resp.Msg.AuthorizeUrl)
	require.NoError(t, err)
	assert.Equal(t,
		"data:read_write,data:delete,project:delete", parsed.Query().Get("scope"),
	)
}

// TestGetTodoistConnectionStatus_TaskAddOnlyNeedsReconnect covers the
// backfilled rows authorized before the pipeline needed projects and deletes.
func TestGetTodoistConnectionStatus_TaskAddOnlyNeedsReconnect(t *testing.T) {
	seedTodoistConnection(t, "task:add")
	client := setupTodoistClient(getRoutes())

	resp, err := client.GetTodoistConnectionStatus(
		newCtx(),
		connect.NewRequest(&learningpathsv1.GetTodoistConnectionStatusRequest{}),
	)
	require.NoError(t, err)
	assert.True(t, resp.Msg.Connected)
	assert.True(t, resp.Msg.NeedsReconnect)
}

func TestGetTodoistSyncState_ReportsProjectAndTasks(t *testing.T) {
	seedTodoistConnection(t, "data:read_write,data:delete,project:delete")
	testSealer, err := crypto.New(testCfg.EncryptionKey)
	require.NoError(t, err)
	repo := repositories.New(testDB, testSealer).LearningPaths
	pathID, itemID := seedPath(t, userID)
	require.NoError(t, repo.SetTodoistProjectID(t.Context(), pathID, "proj-1"))
	require.NoError(t, repo.SetItemTodoistTaskID(t.Context(), itemID, "task-1"))

	client := setupTodoistClient(getRoutes())
	resp, err := client.GetTodoistSyncState(newCtx(), connect.NewRequest(
		&learningpathsv1.GetTodoistSyncStateRequest{LearningPathId: pathID.String()},
	))
	require.NoError(t, err)

	assert.True(t, resp.Msg.Connected)
	assert.False(t, resp.Msg.NeedsReconnect)
	assert.Equal(t, "proj-1", resp.Msg.TodoistProjectId)
	assert.Equal(t, "M1", resp.Msg.ActiveModule)
	require.Len(t, resp.Msg.Items, 1)
	assert.Equal(t, itemID.String(), resp.Msg.Items[0].ItemId)
	assert.Equal(t, "task-1", resp.Msg.Items[0].TodoistTaskId)
}

func TestGetTodoistSyncState_Errors(t *testing.T) {
	client := setupTodoistClient(getRoutes())

	_, err := client.GetTodoistSyncState(newCtx(), connect.NewRequest(
		&learningpathsv1.GetTodoistSyncStateRequest{LearningPathId: "not-a-uuid"},
	))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	foreignPath, _ := seedPath(t, "someone-else")
	_, err = client.GetTodoistSyncState(newCtx(), connect.NewRequest(
		&learningpathsv1.GetTodoistSyncStateRequest{LearningPathId: foreignPath.String()},
	))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	_, err = client.GetTodoistSyncState(newCtx(), connect.NewRequest(
		&learningpathsv1.GetTodoistSyncStateRequest{LearningPathId: uuid.NewString()},
	))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
