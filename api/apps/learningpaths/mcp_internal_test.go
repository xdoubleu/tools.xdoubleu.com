package learningpaths

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/crypto"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mcptools"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/testhelper"
)

func mcpCtx(userID string) context.Context {
	return context.WithValue(
		context.Background(),
		constants.UserContextKey,
		//nolint:exhaustruct // only ID and AppAccess matter for the gate
		sharedmodels.User{ID: userID, AppAccess: []string{mcpAppName}},
	)
}

func mcpMsgJSON(t *testing.T, msg proto.Message) string {
	t.Helper()
	data, err := protojson.Marshal(msg)
	require.NoError(t, err)
	return string(data)
}

// TestMCPTools_ReadAndWrite exercises all six tool wrappers end to end
// against a real app instance sharing the test database TestMain (in
// app_test.go, part of the same compiled test binary) already migrated. The
// three write tools are this app's deliberate exception to the read-only MCP
// convention (see adr-0023): create/update a path and toggle an item, then
// confirm the read tools see those exact changes — proving the write tools
// persist through the same service layer the Connect RPCs use, not some
// MCP-only side path.
func TestMCPTools_ReadAndWrite(t *testing.T) {
	cfg := testhelper.NewTestConfig()
	var pg postgres.DB = testhelper.ConnectTestDB(cfg.DBDsn)

	testSealer, err := crypto.New(cfg.EncryptionKey)
	require.NoError(t, err)

	const mcpUserID = "mcp-learningpaths-user"
	app := New(
		sharedmocks.NewMockedAuthService(mcpUserID),
		logging.NewNopLogger(),
		cfg,
		pg,
		testSealer,
	)
	h := &learningPathsConnectHandler{app: app}

	ctx := mcpCtx(mcpUserID)

	createdMsg, err := h.mcpCreatePath(ctx, mcpCreatePathArgs{
		Title:   "Learn Go",
		Goal:    "Ship a backend service",
		Routine: "30 min every weekday",
		Modules: []mcpModuleArg{
			{
				Title: "Month 1",
				Items: []mcpItemArg{
					{Type: "read", Description: "Read the tour of Go", Completed: false},
					{Type: "do", Description: "Write a CLI tool", Completed: false},
				},
			},
		},
		Resources: []mcpResourceArg{{Text: "https://go.dev/tour/"}},
	})
	require.NoError(t, err)
	created, ok := createdMsg.(*learningpathsv1.CreateLearningPathResponse)
	require.True(t, ok)
	assert.Equal(t, "Learn Go", created.LearningPath.Title)
	pathID := created.LearningPath.Id

	gotMsg, err := h.mcpGetPath(ctx, mcpPathIDArgs{ID: pathID})
	require.NoError(t, err)
	got, ok := gotMsg.(*learningpathsv1.GetLearningPathResponse)
	require.True(t, ok)
	assert.Equal(t, "Learn Go", got.LearningPath.Title)

	listMsg, err := h.mcpListPaths(ctx, mcpListPathsArgs{Limit: 0, Offset: 0})
	require.NoError(t, err)
	assert.Contains(t, mcpMsgJSON(t, listMsg), pathID)

	updatedMsg, err := h.mcpUpdatePath(ctx, mcpUpdatePathArgs{
		ID:      pathID,
		Title:   "Learn Go, Well",
		Goal:    "",
		Routine: "",
		Modules: []mcpModuleArg{
			{
				Title: "Month 1",
				Items: []mcpItemArg{
					{Type: "read", Description: "Read the tour of Go", Completed: false},
				},
			},
		},
		Resources: nil,
	})
	require.NoError(t, err)
	updated, ok := updatedMsg.(*learningpathsv1.UpdateLearningPathResponse)
	require.True(t, ok)
	assert.Equal(t, "Learn Go, Well", updated.LearningPath.Title)
	itemID := updated.LearningPath.Modules[0].Items[0].Id

	progressMsg, err := h.mcpGetProgress(ctx, mcpPathIDArgs{ID: pathID})
	require.NoError(t, err)
	progress, ok := progressMsg.(*learningpathsv1.GetLearningPathProgressResponse)
	require.True(t, ok)
	assert.Equal(t, int32(1), progress.TotalItems)
	assert.Equal(t, int32(0), progress.CompletedItems)

	_, err = h.mcpRecordProgress(
		ctx, mcpRecordProgressArgs{ItemID: itemID, Completed: true},
	)
	require.NoError(t, err)

	progressMsg, err = h.mcpGetProgress(ctx, mcpPathIDArgs{ID: pathID})
	require.NoError(t, err)
	progress, ok = progressMsg.(*learningpathsv1.GetLearningPathProgressResponse)
	require.True(t, ok)
	assert.Equal(t, int32(1), progress.CompletedItems)
}

// TestMCPTools_RequireAppAccess proves the gate rejects a caller without
// learningpaths access, the same way every other app's tools do — including
// the mutating ones, which are this app's exception to the read-only
// convention but not to the access gate.
func TestMCPTools_RequireAppAccess(t *testing.T) {
	ctx := context.WithValue(
		context.Background(),
		constants.UserContextKey,
		//nolint:exhaustruct // a user with no app access at all
		sharedmodels.User{ID: "no-access"},
	)
	require.Error(t, mcptools.RequireAppAccess(ctx, mcpAppName))
}
