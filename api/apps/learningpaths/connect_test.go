package learningpaths_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
	"tools.xdoubleu.com/gen/learningpaths/v1/learningpathsv1connect"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func setupClient(
	handler http.Handler,
) learningpathsv1connect.LearningPathsServiceClient {
	ts := httptest.NewServer(handler)
	return learningpathsv1connect.NewLearningPathsServiceClient(
		http.DefaultClient,
		ts.URL,
	)
}

func contextWithUser(ctx context.Context, user *sharedmodels.User) context.Context {
	return context.WithValue(ctx, constants.UserContextKey, user)
}

func connectErr(err error) *connect.Error {
	target := &connect.Error{}
	_ = errors.As(err, &target)
	return target
}

func newCtx() context.Context {
	return contextWithUser(
		context.Background(),
		&sharedmodels.User{ID: userID}, //nolint:exhaustruct // only ID needed
	)
}

func TestListLearningPaths_Empty(t *testing.T) {
	client := setupClient(getRoutes())

	resp, err := client.ListLearningPaths(
		newCtx(), connect.NewRequest(&learningpathsv1.ListLearningPathsRequest{}),
	)
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.LearningPaths)
}

func TestCreateLearningPath_Success(t *testing.T) {
	client := setupClient(getRoutes())

	resp, err := client.CreateLearningPath(
		newCtx(),
		connect.NewRequest(&learningpathsv1.CreateLearningPathRequest{
			Title:   "Learn Go",
			Goal:    "Ship a backend service",
			Routine: "30 min every weekday",
			Modules: []*learningpathsv1.Module{
				{
					Title: "Month 1",
					Items: []*learningpathsv1.Item{
						{Type: "read", Description: "Read the tour of Go"},
						{Type: "do", Description: "Write a CLI tool"},
					},
				},
			},
			Resources: []*learningpathsv1.Resource{
				{Text: "https://go.dev/tour/"},
			},
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, "Learn Go", resp.Msg.LearningPath.Title)
	assert.Equal(t, userID, resp.Msg.LearningPath.UserId)
	require.Len(t, resp.Msg.LearningPath.Modules, 1)
	assert.Len(t, resp.Msg.LearningPath.Modules[0].Items, 2)
	require.Len(t, resp.Msg.LearningPath.Resources, 1)
	assert.Equal(t, "https://go.dev/tour/", resp.Msg.LearningPath.Resources[0].Text)
}

func TestGetLearningPath_Success(t *testing.T) {
	client := setupClient(getRoutes())
	ctx := newCtx()

	createResp, err := client.CreateLearningPath(
		ctx,
		connect.NewRequest(&learningpathsv1.CreateLearningPathRequest{
			Title: "Test Path",
			Modules: []*learningpathsv1.Module{
				{
					Title: "Week 1",
					Items: []*learningpathsv1.Item{{Description: "Do the thing"}},
				},
			},
		}),
	)
	require.NoError(t, err)

	getResp, err := client.GetLearningPath(
		ctx, connect.NewRequest(&learningpathsv1.GetLearningPathRequest{
			Id: createResp.Msg.LearningPath.Id,
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, "Test Path", getResp.Msg.LearningPath.Title)
	require.Len(t, getResp.Msg.LearningPath.Modules, 1)
	assert.Equal(t, "Week 1", getResp.Msg.LearningPath.Modules[0].Title)
}

func TestGetLearningPath_WithResources(t *testing.T) {
	client := setupClient(getRoutes())
	ctx := newCtx()

	createResp, err := client.CreateLearningPath(
		ctx,
		connect.NewRequest(&learningpathsv1.CreateLearningPathRequest{
			Title: "Resourced Path",
			Resources: []*learningpathsv1.Resource{
				{Text: "https://example.com/a"},
				{Text: "https://example.com/b"},
			},
		}),
	)
	require.NoError(t, err)

	getResp, err := client.GetLearningPath(
		ctx, connect.NewRequest(&learningpathsv1.GetLearningPathRequest{
			Id: createResp.Msg.LearningPath.Id,
		}),
	)
	require.NoError(t, err)
	require.Len(t, getResp.Msg.LearningPath.Resources, 2)
	assert.Equal(t, "https://example.com/a", getResp.Msg.LearningPath.Resources[0].Text)
	assert.Equal(t, "https://example.com/b", getResp.Msg.LearningPath.Resources[1].Text)
}

func TestGetLearningPath_NotFound(t *testing.T) {
	client := setupClient(getRoutes())

	_, err := client.GetLearningPath(
		newCtx(), connect.NewRequest(&learningpathsv1.GetLearningPathRequest{
			Id: uuid.New().String(),
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connectErr(err).Code())
}

// TestGetLearningPath_OtherUserDenied stages a path belonging to a different
// user directly in the database and confirms userID gets a not-found rather
// than a forbidden — per-user scoping has no sharing concept, so foreign
// ownership should read the same as a missing ID.
func TestGetLearningPath_OtherUserDenied(t *testing.T) {
	client := setupClient(getRoutes())

	var pathID string
	err := testDB.QueryRow(context.Background(), `
		INSERT INTO learningpaths.learning_paths (user_id, title)
		VALUES ('no-access-owner', 'Hidden Path')
		RETURNING id::text`,
	).Scan(&pathID)
	require.NoError(t, err)

	_, err = client.GetLearningPath(
		newCtx(),
		connect.NewRequest(&learningpathsv1.GetLearningPathRequest{Id: pathID}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connectErr(err).Code())
}

func TestUpdateLearningPath_Success(t *testing.T) {
	client := setupClient(getRoutes())
	ctx := newCtx()

	createResp, err := client.CreateLearningPath(
		ctx,
		connect.NewRequest(&learningpathsv1.CreateLearningPathRequest{
			Title: "Original",
			Modules: []*learningpathsv1.Module{
				{Title: "M1", Items: []*learningpathsv1.Item{{Description: "item 1"}}},
			},
		}),
	)
	require.NoError(t, err)
	pathID := createResp.Msg.LearningPath.Id

	updateResp, err := client.UpdateLearningPath(
		ctx, connect.NewRequest(&learningpathsv1.UpdateLearningPathRequest{
			Id:    pathID,
			Title: "Updated",
			Modules: []*learningpathsv1.Module{
				{Title: "M1 renamed", Items: []*learningpathsv1.Item{
					{Description: "item 1"}, {Description: "item 2"},
				}},
			},
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, "Updated", updateResp.Msg.LearningPath.Title)
	require.Len(t, updateResp.Msg.LearningPath.Modules, 1)
	assert.Equal(t, "M1 renamed", updateResp.Msg.LearningPath.Modules[0].Title)
	assert.Len(t, updateResp.Msg.LearningPath.Modules[0].Items, 2)
}

func TestUpdateLearningPath_NotFound(t *testing.T) {
	client := setupClient(getRoutes())

	_, err := client.UpdateLearningPath(
		newCtx(), connect.NewRequest(&learningpathsv1.UpdateLearningPathRequest{
			Id: uuid.New().String(), Title: "ghost",
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connectErr(err).Code())
}

func TestDeleteLearningPath_Success(t *testing.T) {
	client := setupClient(getRoutes())
	ctx := newCtx()

	createResp, err := client.CreateLearningPath(
		ctx,
		connect.NewRequest(
			&learningpathsv1.CreateLearningPathRequest{Title: "To Delete"},
		),
	)
	require.NoError(t, err)
	pathID := createResp.Msg.LearningPath.Id

	_, err = client.DeleteLearningPath(
		ctx, connect.NewRequest(&learningpathsv1.DeleteLearningPathRequest{Id: pathID}),
	)
	require.NoError(t, err)

	_, err = client.GetLearningPath(
		ctx, connect.NewRequest(&learningpathsv1.GetLearningPathRequest{Id: pathID}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connectErr(err).Code())
}

func TestDeleteLearningPath_NotFound(t *testing.T) {
	client := setupClient(getRoutes())

	_, err := client.DeleteLearningPath(
		newCtx(), connect.NewRequest(&learningpathsv1.DeleteLearningPathRequest{
			Id: uuid.New().String(),
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connectErr(err).Code())
}

func TestRecordItemProgress_TogglesCompleted(t *testing.T) {
	client := setupClient(getRoutes())
	ctx := newCtx()

	createResp, err := client.CreateLearningPath(
		ctx, connect.NewRequest(&learningpathsv1.CreateLearningPathRequest{
			Title: "Progress Path",
			Modules: []*learningpathsv1.Module{
				{
					Title: "M1",
					Items: []*learningpathsv1.Item{{Description: "check me"}},
				},
			},
		}),
	)
	require.NoError(t, err)
	itemID := createResp.Msg.LearningPath.Modules[0].Items[0].Id
	assert.False(t, createResp.Msg.LearningPath.Modules[0].Items[0].Completed)

	_, err = client.RecordItemProgress(
		ctx, connect.NewRequest(&learningpathsv1.RecordItemProgressRequest{
			ItemId: itemID, Completed: true,
		}),
	)
	require.NoError(t, err)

	getResp, err := client.GetLearningPath(
		ctx, connect.NewRequest(&learningpathsv1.GetLearningPathRequest{
			Id: createResp.Msg.LearningPath.Id,
		}),
	)
	require.NoError(t, err)
	assert.True(t, getResp.Msg.LearningPath.Modules[0].Items[0].Completed)
}

func TestRecordItemProgress_OtherUserDenied(t *testing.T) {
	client := setupClient(getRoutes())

	var pathID, moduleID, itemID string
	err := testDB.QueryRow(context.Background(), `
		INSERT INTO learningpaths.learning_paths (user_id, title)
		VALUES ('no-access-owner', 'Hidden Path')
		RETURNING id::text`,
	).Scan(&pathID)
	require.NoError(t, err)
	err = testDB.QueryRow(context.Background(), `
		INSERT INTO learningpaths.modules (learning_path_id, title)
		VALUES ($1, 'M1')
		RETURNING id::text`,
		pathID,
	).Scan(&moduleID)
	require.NoError(t, err)
	err = testDB.QueryRow(context.Background(), `
		INSERT INTO learningpaths.items (module_id, description)
		VALUES ($1, 'secret item')
		RETURNING id::text`,
		moduleID,
	).Scan(&itemID)
	require.NoError(t, err)

	_, err = client.RecordItemProgress(
		newCtx(), connect.NewRequest(&learningpathsv1.RecordItemProgressRequest{
			ItemId: itemID, Completed: true,
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connectErr(err).Code())
}

func TestGetLearningPath_InvalidID(t *testing.T) {
	client := setupClient(getRoutes())

	_, err := client.GetLearningPath(
		newCtx(),
		connect.NewRequest(&learningpathsv1.GetLearningPathRequest{Id: "not-a-uuid"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestUpdateLearningPath_InvalidID(t *testing.T) {
	client := setupClient(getRoutes())

	_, err := client.UpdateLearningPath(
		newCtx(),
		connect.NewRequest(
			&learningpathsv1.UpdateLearningPathRequest{Id: "not-a-uuid"},
		),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestDeleteLearningPath_InvalidID(t *testing.T) {
	client := setupClient(getRoutes())

	_, err := client.DeleteLearningPath(
		newCtx(),
		connect.NewRequest(
			&learningpathsv1.DeleteLearningPathRequest{Id: "not-a-uuid"},
		),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestRecordItemProgress_InvalidID(t *testing.T) {
	client := setupClient(getRoutes())

	_, err := client.RecordItemProgress(
		newCtx(),
		connect.NewRequest(
			&learningpathsv1.RecordItemProgressRequest{ItemId: "not-a-uuid"},
		),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestListLearningPaths_Pagination(t *testing.T) {
	client := setupClient(getRoutes())
	ctx := newCtx()

	baseline, err := client.ListLearningPaths(
		ctx, connect.NewRequest(&learningpathsv1.ListLearningPathsRequest{Limit: 1000}),
	)
	require.NoError(t, err)
	existing := int32(len(baseline.Msg.LearningPaths))

	for _, title := range []string{
		"zzz_paginated_one", "zzz_paginated_two", "zzz_paginated_three",
	} {
		_, err = client.CreateLearningPath(
			ctx,
			connect.NewRequest(
				&learningpathsv1.CreateLearningPathRequest{Title: title},
			),
		)
		require.NoError(t, err)
	}

	firstPage, err := client.ListLearningPaths(
		ctx, connect.NewRequest(&learningpathsv1.ListLearningPathsRequest{
			Limit: 2, Offset: existing,
		}),
	)
	require.NoError(t, err)
	assert.Len(t, firstPage.Msg.LearningPaths, 2)
	assert.True(t, firstPage.Msg.HasMore)

	secondPage, err := client.ListLearningPaths(
		ctx, connect.NewRequest(&learningpathsv1.ListLearningPathsRequest{
			Limit: 2, Offset: existing + 2,
		}),
	)
	require.NoError(t, err)
	assert.Len(t, secondPage.Msg.LearningPaths, 1)
	assert.False(t, secondPage.Msg.HasMore)
}

func TestGetLearningPathProgress_Success(t *testing.T) {
	client := setupClient(getRoutes())
	ctx := newCtx()

	createResp, err := client.CreateLearningPath(
		ctx, connect.NewRequest(&learningpathsv1.CreateLearningPathRequest{
			Title: "Progress Overview",
			Modules: []*learningpathsv1.Module{
				{
					Title: "M1",
					Items: []*learningpathsv1.Item{
						{Description: "one", Completed: true},
						{Description: "two"},
					},
				},
				{
					Title: "M2",
					Items: []*learningpathsv1.Item{
						{Description: "three"},
					},
				},
			},
		}),
	)
	require.NoError(t, err)

	resp, err := client.GetLearningPathProgress(
		ctx, connect.NewRequest(&learningpathsv1.GetLearningPathProgressRequest{
			Id: createResp.Msg.LearningPath.Id,
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, int32(3), resp.Msg.TotalItems)
	assert.Equal(t, int32(1), resp.Msg.CompletedItems)
	require.Len(t, resp.Msg.Modules, 2)
	assert.Equal(t, int32(2), resp.Msg.Modules[0].TotalItems)
	assert.Equal(t, int32(1), resp.Msg.Modules[0].CompletedItems)
	assert.Equal(t, int32(1), resp.Msg.Modules[1].TotalItems)
	assert.Equal(t, int32(0), resp.Msg.Modules[1].CompletedItems)
}

func TestGetLearningPathProgress_NotFound(t *testing.T) {
	client := setupClient(getRoutes())

	_, err := client.GetLearningPathProgress(
		newCtx(), connect.NewRequest(&learningpathsv1.GetLearningPathProgressRequest{
			Id: uuid.NewString(),
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connectErr(err).Code())
}

func TestGetLearningPathProgress_OtherUserDenied(t *testing.T) {
	client := setupClient(getRoutes())

	var pathID string
	err := testDB.QueryRow(context.Background(), `
		INSERT INTO learningpaths.learning_paths (user_id, title)
		VALUES ('no-access-owner-progress', 'Hidden Progress Path')
		RETURNING id::text`,
	).Scan(&pathID)
	require.NoError(t, err)

	_, err = client.GetLearningPathProgress(
		newCtx(), connect.NewRequest(&learningpathsv1.GetLearningPathProgressRequest{
			Id: pathID,
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connectErr(err).Code())
}

func TestGetLearningPathProgress_InvalidID(t *testing.T) {
	client := setupClient(getRoutes())

	_, err := client.GetLearningPathProgress(
		newCtx(),
		connect.NewRequest(
			&learningpathsv1.GetLearningPathProgressRequest{Id: "not-a-uuid"},
		),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}
