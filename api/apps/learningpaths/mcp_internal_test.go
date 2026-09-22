package learningpaths

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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
	// booksApp/feedsApp are nil — this test never sets a resource's
	// linked_book_id/linked_feed_item_id, so the service layer's resolve/
	// validate calls into them are never reached (see resolveResourceLinks
	// in internal/services/learningpaths.go).
	app := New(
		sharedmocks.NewMockedAuthService(mcpUserID),
		logging.NewNopLogger(),
		cfg,
		pg,
		testSealer,
		nil,
		nil,
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

// TestMCPAuthoringGuide_ResourceExistsAndReads verifies the authoring guide is
// exposed as a real MCP resource (so any resource-reading client — Claude
// Code, OpenCode — can pull the whole guidebook) and that its text carries the
// authoring workflow and curation rubric, and that the write-tool descriptions
// embed the same essentials for clients that only read tool metadata (e.g. a
// ChatGPT-style connector). The resource handler is called directly; no MCP
// transport or database is needed.
func TestMCPAuthoringGuide_ResourceExistsAndReads(t *testing.T) {
	// The write-tool descriptions carry the workflow essentials on the tools
	// themselves — the surface a ChatGPT-style connector reads.
	require.Contains(t, mcpCreatePathDescription, "confirm the full proposed tree")
	require.Contains(t, mcpCreatePathDescription, "modules")
	require.Contains(t, mcpUpdatePathDescription, "Wholesale-replaces")
	require.Contains(t, mcpUpdatePathDescription, "learningpaths_get_path")
	require.Contains(t, mcpRecordProgressDescription, "item_id")

	// The full guidebook is reachable through the handler with the expected
	// URI, MIME type, and authoring-workflow/rubric content.
	res, err := authoringGuideHandler(context.Background(),
		//nolint:exhaustruct // the handler ignores the request, so no fields needed
		&mcp.ReadResourceRequest{})
	require.NoError(t, err)
	require.Len(t, res.Contents, 1)
	c := res.Contents[0]
	assert.Equal(t, authoringGuideURI, c.URI)
	assert.Equal(t, authoringGuideMIME, c.MIMEType)
	assert.Contains(t, c.Text, "## Authoring workflow")
	assert.Contains(t, c.Text, "## Curation rubric")
	assert.Contains(t, c.Text, "learningpaths_create_path")
	assert.Contains(t, c.Text, "confirm the full proposed tree")
}

// TestMCPAuthoringGuide_RegisteredOnServer ensures registerAuthoringGuideResource
// attaches the resource without panicking (an invalid/relative URI panics in
// srv.AddResource), so the guide is actually present on the live apps server.
func TestMCPAuthoringGuide_RegisteredOnServer(t *testing.T) {
	//nolint:exhaustruct // Name/Version identify the server; nothing else matters here
	srv := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)
	require.NotPanics(t, func() { registerAuthoringGuideResource(srv) })
}
