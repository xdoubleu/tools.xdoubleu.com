package learningpaths_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
)

func setPaused(
	ctx context.Context, id string, paused bool,
) (*connect.Response[learningpathsv1.SetLearningPathPausedResponse], error) {
	return setupClient(getRoutes()).SetLearningPathPaused(
		ctx, connect.NewRequest(&learningpathsv1.SetLearningPathPausedRequest{
			Id: id, Paused: paused,
		}),
	)
}

func TestSetLearningPathPaused_PauseAndResume(t *testing.T) {
	client := setupClient(getRoutes())
	ctx := newCtx()

	createResp, err := client.CreateLearningPath(
		ctx, connect.NewRequest(&learningpathsv1.CreateLearningPathRequest{
			Title: "Pausable",
			Modules: []*learningpathsv1.Module{
				{Title: "M1", Items: []*learningpathsv1.Item{{Description: "one"}}},
			},
		}),
	)
	require.NoError(t, err)
	id := createResp.Msg.LearningPath.Id
	// Mock auth shares one user across tests, so leave its path list as found.
	t.Cleanup(func() {
		_, _ = client.DeleteLearningPath(
			ctx, connect.NewRequest(&learningpathsv1.DeleteLearningPathRequest{Id: id}),
		)
	})
	assert.False(t, createResp.Msg.LearningPath.Paused)

	resp, err := setPaused(ctx, id, true)
	require.NoError(t, err)
	assert.True(t, resp.Msg.LearningPath.Paused)
	assert.Len(t, resp.Msg.LearningPath.Modules, 1)

	// A full-tree update leaves the path paused.
	_, err = client.UpdateLearningPath(
		ctx, connect.NewRequest(&learningpathsv1.UpdateLearningPathRequest{
			Id: id, Title: "Pausable (edited)",
		}),
	)
	require.NoError(t, err)

	listResp, err := client.ListLearningPaths(
		ctx, connect.NewRequest(&learningpathsv1.ListLearningPathsRequest{Limit: 100}),
	)
	require.NoError(t, err)
	var listed *learningpathsv1.LearningPath
	for _, lp := range listResp.Msg.LearningPaths {
		if lp.Id == id {
			listed = lp
		}
	}
	require.NotNil(t, listed)
	assert.True(t, listed.Paused)

	resp, err = setPaused(ctx, id, false)
	require.NoError(t, err)
	assert.False(t, resp.Msg.LearningPath.Paused)
}

func TestSetLearningPathPaused_OtherUserDenied(t *testing.T) {
	var pathID string
	err := testDB.QueryRow(context.Background(), `
		INSERT INTO learningpaths.learning_paths (user_id, title)
		VALUES ('pause-owner', 'Not Yours')
		RETURNING id::text`,
	).Scan(&pathID)
	require.NoError(t, err)

	_, err = setPaused(newCtx(), pathID, true)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connectErr(err).Code())

	var paused bool
	err = testDB.QueryRow(context.Background(),
		`SELECT paused FROM learningpaths.learning_paths WHERE id = $1`, pathID,
	).Scan(&paused)
	require.NoError(t, err)
	assert.False(t, paused)
}

func TestSetLearningPathPaused_NotFound(t *testing.T) {
	_, err := setPaused(newCtx(), uuid.NewString(), true)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connectErr(err).Code())
}

func TestSetLearningPathPaused_InvalidID(t *testing.T) {
	_, err := setPaused(newCtx(), "not-a-uuid", true)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}
