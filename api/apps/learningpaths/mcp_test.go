//nolint:exhaustruct // sparse tool arguments
package learningpaths_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
	"tools.xdoubleu.com/internal/kit/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

type mcpUser struct {
	t       *testing.T
	session *mcp.ClientSession
}

// newMCPUser serves the app's tools as uid; access lists the apps uid may use.
func newMCPUser(t *testing.T, uid string, access ...string) mcpUser {
	t.Helper()
	ctx := t.Context()

	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	testApp.RegisterMCPTools(srv)
	srv.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			user := sharedmodels.User{ID: uid, AppAccess: access}
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
	return protojson.UnmarshalOptions{DiscardUnknown: true}.
		Unmarshal([]byte(text.Text), out)
}

func newLPUser(t *testing.T, uid string) mcpUser {
	t.Helper()
	return newMCPUser(t, uid, "learningpaths")
}

// TestMCPTools_ReadAndWrite exercises the tools against a real app: the write
// tools' changes must be visible through the read tools, proving they persist
// through the same service layer as the Connect RPCs.
func TestMCPTools_ReadAndWrite(t *testing.T) {
	u := newLPUser(t, "mcp-learningpaths-user")

	var created learningpathsv1.CreateLearningPathResponse
	require.NoError(t, u.call("learningpaths_create_path", map[string]any{
		"title":   "Learn Go",
		"goal":    "Ship a backend service",
		"routine": "30 min every weekday",
		"modules": []any{map[string]any{
			"title": "Month 1",
			"items": []any{
				map[string]any{"type": "read", "description": "Read the tour of Go"},
				map[string]any{"type": "do", "description": "Write a CLI tool"},
			},
		}},
		"resources": []any{map[string]any{"text": "https://go.dev/tour/"}},
	}, &created))
	assert.Equal(t, "Learn Go", created.LearningPath.Title)
	pathID := created.LearningPath.Id

	var got learningpathsv1.GetLearningPathResponse
	require.NoError(t, u.call(
		"learningpaths_get_path", map[string]any{"id": pathID}, &got,
	))
	assert.Equal(t, "Learn Go", got.LearningPath.Title)

	var list learningpathsv1.ListLearningPathsResponse
	require.NoError(t, u.call("learningpaths_list_paths", map[string]any{}, &list))
	ids := make([]string, len(list.LearningPaths))
	for i, p := range list.LearningPaths {
		ids[i] = p.Id
	}
	assert.Contains(t, ids, pathID)

	var updated learningpathsv1.UpdateLearningPathResponse
	require.NoError(t, u.call("learningpaths_update_path", map[string]any{
		"id":    pathID,
		"title": "Learn Go, Well",
		"modules": []any{map[string]any{
			"title": "Month 1",
			"items": []any{
				map[string]any{"type": "read", "description": "Read the tour of Go"},
			},
		}},
	}, &updated))
	assert.Equal(t, "Learn Go, Well", updated.LearningPath.Title)
	itemID := updated.LearningPath.Modules[0].Items[0].Id

	var progress learningpathsv1.GetLearningPathProgressResponse
	require.NoError(t, u.call(
		"learningpaths_get_progress", map[string]any{"id": pathID}, &progress,
	))
	assert.Equal(t, int32(1), progress.TotalItems)
	assert.Equal(t, int32(0), progress.CompletedItems)

	var recorded learningpathsv1.RecordItemProgressResponse
	require.NoError(t, u.call("learningpaths_record_progress", map[string]any{
		"item_id": itemID, "completed": true,
	}, &recorded))

	progress.Reset()
	require.NoError(t, u.call(
		"learningpaths_get_progress", map[string]any{"id": pathID}, &progress,
	))
	assert.Equal(t, int32(1), progress.CompletedItems)
	require.Len(t, progress.Modules, 1)
	assert.Equal(t, int32(1), progress.Modules[0].CompletedItems)

	var state learningpathsv1.GetTodoistSyncStateResponse
	require.NoError(t, u.call(
		"learningpaths_get_todoist_status", map[string]any{"id": pathID}, &state,
	))
	assert.False(t, state.Connected)
}

func TestMCPTools_InvalidIDs(t *testing.T) {
	u := newLPUser(t, "mcp-invalid-id-user")
	var out learningpathsv1.GetLearningPathResponse
	require.Error(t, u.call(
		"learningpaths_get_path", map[string]any{"id": "nope"}, &out,
	))
	var rec learningpathsv1.RecordItemProgressResponse
	require.Error(t, u.call("learningpaths_record_progress", map[string]any{
		"item_id": "nope", "completed": true,
	}, &rec))
	require.Error(t, u.call("learningpaths_get_path", map[string]any{
		"id": uuid.NewString(),
	}, &out))
}

// TestMCPTools_RequireAppAccess: every tool, mutating ones included, rejects
// a caller without learningpaths access.
func TestMCPTools_RequireAppAccess(t *testing.T) {
	u := newMCPUser(t, "no-access")
	var list learningpathsv1.ListLearningPathsResponse
	require.Error(t, u.call("learningpaths_list_paths", map[string]any{}, &list))
	var created learningpathsv1.CreateLearningPathResponse
	require.Error(t, u.call(
		"learningpaths_create_path", map[string]any{"title": "x"}, &created,
	))
}

// TestMCPQuiz_RoundTrip: a module's quiz survives create_path -> get_path and
// gets replaced by update_path, proving the JSONB column round-trips through
// the same service layer as the Connect RPCs.
func TestMCPQuiz_RoundTrip(t *testing.T) {
	u := newLPUser(t, "mcp-quiz-user")

	quiz := func(prompt string, options []any, correct int) []any {
		return []any{map[string]any{
			"prompt": prompt, "options": options, "correct_answer_index": correct,
		}}
	}
	var created learningpathsv1.CreateLearningPathResponse
	require.NoError(t, u.call("learningpaths_create_path", map[string]any{
		"title": "Learn Go",
		"modules": []any{map[string]any{
			"title": "Basics",
			"items": []any{
				map[string]any{"type": "read", "description": "Read the tour"},
				map[string]any{"type": "checkpoint", "description": "Pass the quiz"},
			},
			"quiz": quiz("What is 2+2?", []any{"3", "4", "5"}, 1),
		}},
	}, &created))
	pathID := created.LearningPath.Id

	var got learningpathsv1.GetLearningPathResponse
	require.NoError(t, u.call(
		"learningpaths_get_path", map[string]any{"id": pathID}, &got,
	))
	require.Len(t, got.LearningPath.Modules, 1)
	gotQuiz := got.LearningPath.Modules[0].Quiz
	require.Len(t, gotQuiz, 1)
	assert.Equal(t, "What is 2+2?", gotQuiz[0].Prompt)
	assert.Equal(t, []string{"3", "4", "5"}, gotQuiz[0].Options)
	assert.Equal(t, int32(1), gotQuiz[0].CorrectAnswerIndex)
	// The quiz is carried by the module, not duplicated per item.
	assert.Len(t, got.LearningPath.Modules[0].Items, 2)

	var updated learningpathsv1.UpdateLearningPathResponse
	require.NoError(t, u.call("learningpaths_update_path", map[string]any{
		"id":    pathID,
		"title": "Learn Go",
		"modules": []any{map[string]any{
			"title": "Basics",
			"quiz":  quiz("Replaced?", []any{"yes", "no"}, 0),
		}},
	}, &updated))
	replaced := updated.LearningPath.Modules[0].Quiz
	require.Len(t, replaced, 1)
	assert.Equal(t, "Replaced?", replaced[0].Prompt)
}

// TestMCPQuiz_MalformedJSONErrors: a module whose quiz column holds invalid
// JSON must surface an error on get_path rather than silently drop it.
func TestMCPQuiz_MalformedJSONErrors(t *testing.T) {
	u := newLPUser(t, "mcp-malformed-quiz-user")

	var created learningpathsv1.CreateLearningPathResponse
	require.NoError(t, u.call("learningpaths_create_path", map[string]any{
		"title": "Corrupt",
		"modules": []any{map[string]any{
			"title": "Broken",
			"items": []any{map[string]any{"type": "read", "description": "x"}},
			"quiz": []any{map[string]any{
				"prompt": "?", "options": []any{"a", "b"}, "correct_answer_index": 0,
			}},
		}},
	}, &created))
	pathID := created.LearningPath.Id

	// A JSON object where an array is expected surfaces an unmarshal error.
	_, err := testDB.Exec(t.Context(),
		`UPDATE learningpaths.modules SET quiz = '{"not":"an array"}'
		 WHERE learning_path_id = $1`, uuid.MustParse(pathID))
	require.NoError(t, err)

	var got learningpathsv1.GetLearningPathResponse
	require.Error(t, u.call(
		"learningpaths_get_path", map[string]any{"id": pathID}, &got,
	))
}
