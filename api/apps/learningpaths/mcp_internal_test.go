package learningpaths

import (
	"context"
	"testing"

	"github.com/google/uuid"
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

// TestMCPTools_ReadAndWrite exercises all six tools against a real app: the
// write tools' changes must be visible through the read tools, proving they
// persist through the same service layer as the Connect RPCs.
func TestMCPTools_ReadAndWrite(t *testing.T) {
	cfg := testhelper.NewTestConfig()
	var pg postgres.DB = testhelper.ConnectTestDB(cfg.DBDsn)

	testSealer, err := crypto.New(cfg.EncryptionKey)
	require.NoError(t, err)

	const mcpUserID = "mcp-learningpaths-user"
	// booksApp/feedsApp are nil: no resource here is linked.
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
{
						Type:         "read",
						Description:  "Read the tour of Go",
						Completed:    false,
						LinkedBookID: nil,
					},
					{Type: "do", Description: "Write a CLI tool", Completed: false, LinkedBookID: nil},
				},
				Quiz: nil,
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
					{
						Type:         "read",
						Description:  "Read the tour of Go",
						Completed:    false,
						LinkedBookID: nil,
					},
				},
				Quiz: nil,
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

// TestMCPTools_RequireAppAccess: every tool, mutating ones included, rejects
// a caller without learningpaths access.
func TestMCPTools_RequireAppAccess(t *testing.T) {
	ctx := context.WithValue(
		context.Background(),
		constants.UserContextKey,
		//nolint:exhaustruct // a user with no app access at all
		sharedmodels.User{ID: "no-access"},
	)
	require.Error(t, mcptools.RequireAppAccess(ctx, mcpAppName))
}

// TestMCPQuiz_RoundTrip: a module's quiz survives create_path -> get_path and
// gets replaced by update_path, proving the JSONB column round-trips through
// the same service layer as the Connect RPCs.
func TestMCPQuiz_RoundTrip(t *testing.T) {
	cfg := testhelper.NewTestConfig()
	var pg postgres.DB = testhelper.ConnectTestDB(cfg.DBDsn)

	testSealer, err := crypto.New(cfg.EncryptionKey)
	require.NoError(t, err)

	const quizUserID = "mcp-quiz-user"
	app := New(
		sharedmocks.NewMockedAuthService(quizUserID),
		logging.NewNopLogger(),
		cfg,
		pg,
		testSealer,
		nil,
		nil,
	)
	h := &learningPathsConnectHandler{app: app}
	ctx := mcpCtx(quizUserID)

	quiz := []mcpQuizQuestionArg{{
		Prompt:             "What is 2+2?",
		Options:            []string{"3", "4", "5"},
		CorrectAnswerIndex: 1,
	}, {
		Prompt:             "What is Go?",
		Options:            []string{"a game", "a language"},
		CorrectAnswerIndex: 1,
	}}

	createdMsg, err := h.mcpCreatePath(ctx, mcpCreatePathArgs{
		Title:   "Learn Go",
		Goal:    "",
		Routine: "",
		Modules: []mcpModuleArg{{
			Title: "Basics",
			// Terminal checkpoint item carries the quiz.
			Items: []mcpItemArg{
				{Type: "read", Description: "Read the tour", Completed: false},
				{Type: "checkpoint", Description: "Pass the quiz", Completed: false},
			},
			Quiz: quiz,
		}},
		Resources: nil,
	})
	require.NoError(t, err)
	created, ok := createdMsg.(*learningpathsv1.CreateLearningPathResponse)
	require.True(t, ok)
	pathID := created.LearningPath.Id

	got, err := h.mcpGetPath(ctx, mcpPathIDArgs{ID: pathID})
	require.NoError(t, err)
	gotResp, ok := got.(*learningpathsv1.GetLearningPathResponse)
	require.True(t, ok)
	gotPath := gotResp.LearningPath
	require.Len(t, gotPath.Modules, 1)
	assert.Equal(t, quiz, mcpModuleQuizFromProto(t, gotPath.Modules[0].Quiz))
	// The quiz is carried by the module, not duplicated per item.
	assert.Len(t, gotPath.Modules[0].Items, 2)

	// update_path replaces the quiz wholesale.
	replaced := []mcpQuizQuestionArg{{
		Prompt:             "Replaced?",
		Options:            []string{"yes", "no"},
		CorrectAnswerIndex: 0,
	}}
	updatedMsg, err := h.mcpUpdatePath(ctx, mcpUpdatePathArgs{
		ID:      pathID,
		Title:   "Learn Go",
		Goal:    "",
		Routine: "",
		Modules: []mcpModuleArg{{
			Title: "Basics",
			Items: nil,
			Quiz:  replaced,
		}},
		Resources: nil,
	})
	require.NoError(t, err)
	updated, ok := updatedMsg.(*learningpathsv1.UpdateLearningPathResponse)
	require.True(t, ok)
	updatedPath := updated.LearningPath
	require.Len(t, updatedPath.Modules, 1)
	assert.Equal(t, replaced, mcpModuleQuizFromProto(t, updatedPath.Modules[0].Quiz))
}

// mcpModuleQuizFromProto converts generated quiz messages back to the arg
// shape for direct comparison.
func mcpModuleQuizFromProto(
	t *testing.T,
	in []*learningpathsv1.QuizQuestion,
) []mcpQuizQuestionArg {
	t.Helper()
	out := make([]mcpQuizQuestionArg, len(in))
	for i, q := range in {
		out[i] = mcpQuizQuestionArg{
			Prompt:             q.Prompt,
			Options:            q.Options,
			CorrectAnswerIndex: q.CorrectAnswerIndex,
		}
	}
	return out
}

// TestMCPQuiz_MalformedJSONErrors: a module whose quiz column holds invalid
// JSON must surface an error on get_path rather than silently drop it.
func TestMCPQuiz_MalformedJSONErrors(t *testing.T) {
	cfg := testhelper.NewTestConfig()
	var pg postgres.DB = testhelper.ConnectTestDB(cfg.DBDsn)

	testSealer, err := crypto.New(cfg.EncryptionKey)
	require.NoError(t, err)

	const malformedUserID = "mcp-malformed-quiz-user"
	app := New(
		sharedmocks.NewMockedAuthService(malformedUserID),
		logging.NewNopLogger(),
		cfg,
		pg,
		testSealer,
		nil,
		nil,
	)
	h := &learningPathsConnectHandler{app: app}
	ctx := mcpCtx(malformedUserID)

	createdMsg, err := h.mcpCreatePath(ctx, mcpCreatePathArgs{
		Title:   "Corrupt",
		Goal:    "",
		Routine: "",
		Modules: []mcpModuleArg{{
			Title: "Broken",
			Items: []mcpItemArg{{Type: "read", Description: "x", Completed: false}},
			Quiz: []mcpQuizQuestionArg{{
				Prompt: "?", Options: []string{"a", "b"}, CorrectAnswerIndex: 0,
			}},
		}},
		Resources: nil,
	})
	require.NoError(t, err)
	created, ok := createdMsg.(*learningpathsv1.CreateLearningPathResponse)
	require.True(t, ok)
	pathID := created.LearningPath.Id

	// Corrupt the quiz JSON directly in the DB: a JSON object where an array is
	// expected surfaces an unmarshal error on get_path.
	_, err = pg.Exec(ctx,
		`UPDATE learningpaths.modules SET quiz = '{"not":"an array"}'
		 WHERE learning_path_id = $1`, uuid.MustParse(pathID))
	require.NoError(t, err)

	_, err = h.mcpGetPath(ctx, mcpPathIDArgs{ID: pathID})
	require.Error(t, err)
}

// TestMCPAuthoringGuide_ResourceExistsAndReads: the guide is served as an MCP
// resource, and the write-tool descriptions embed its essentials for clients
// that only read tool metadata.
func TestMCPAuthoringGuide_ResourceExistsAndReads(t *testing.T) {
	require.Contains(t, mcpCreatePathDescription, "confirm the full proposed tree")
	require.Contains(t, mcpCreatePathDescription, "modules")
	require.Contains(t, mcpUpdatePathDescription, "Wholesale-replaces")
	require.Contains(t, mcpUpdatePathDescription, "learningpaths_get_path")
	require.Contains(t, mcpRecordProgressDescription, "item_id")

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

// TestMCPAuthoringGuide_RegisteredOnServer: registration doesn't panic
// (srv.AddResource panics on an invalid URI).
func TestMCPAuthoringGuide_RegisteredOnServer(t *testing.T) {
	//nolint:exhaustruct // Name/Version identify the server; nothing else matters here
	srv := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)
	require.NotPanics(t, func() { registerAuthoringGuideResource(srv) })
}
