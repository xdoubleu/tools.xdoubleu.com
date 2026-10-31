package factory_test

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/constants"
)

func create(
	t *testing.T,
	c factoryv1connect.TaskServiceClient,
	title string,
) *factoryv1.Task {
	t.Helper()
	resp, err := c.CreateTask(
		context.Background(),
		&factoryv1.CreateTaskRequest{Title: title, Body: "body of " + title},
	)
	require.NoError(t, err)
	return resp.Task
}

func move(
	c factoryv1connect.TaskServiceClient,
	id string,
	status factoryv1.TaskStatus,
) (*factoryv1.Task, error) {
	resp, err := c.MoveTask(
		context.Background(),
		&factoryv1.MoveTaskRequest{Id: id, Status: status},
	)
	if err != nil {
		return nil, err
	}
	return resp.Task, nil
}

func TestCreateStartsInBacklogWithDefaults(t *testing.T) {
	c := browserClient(t)

	resp, err := c.CreateTask(context.Background(), &factoryv1.CreateTaskRequest{
		Title:  "  Fix the thing  ",
		Labels: []string{"bug", " bug ", "", "ui"},
	})
	require.NoError(t, err)

	task := resp.Task
	assert.Equal(t, "Fix the thing", task.Title)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_BACKLOG, task.Status)
	assert.Equal(t, factoryv1.TaskPriority_TASK_PRIORITY_P2, task.Priority)
	assert.Equal(t, []string{"bug", "ui"}, task.Labels)
	assert.Equal(t, "human:"+adminID, task.CreatedBy)
}

func TestCreateValidation(t *testing.T) {
	c := browserClient(t)
	for name, req := range map[string]*factoryv1.CreateTaskRequest{
		"empty title": {Title: "   "},
		"long title":  {Title: strings.Repeat("a", 201)},
		"long body":   {Title: "x", Body: strings.Repeat("a", 20001)},
		"long label":  {Title: "x", Labels: []string{strings.Repeat("a", 41)}},
		"many labels": {
			Title: "x",
			Labels: []string{
				"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11",
			},
		},
		"bad priority": {Title: "x", Priority: 99},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := c.CreateTask(context.Background(), req)
			assert.Equal(t, connect.CodeInvalidArgument, code(err))
		})
	}
}

func TestListOmitsBodyGetKeepsIt(t *testing.T) {
	c := browserClient(t)
	task := create(t, c, "one")

	list, err := c.ListTasks(context.Background(), &factoryv1.ListTasksRequest{})
	require.NoError(t, err)
	require.Len(t, list.Tasks, 1)
	assert.Empty(t, list.Tasks[0].Body)

	got, err := c.GetTask(context.Background(), &factoryv1.GetTaskRequest{Id: task.Id})
	require.NoError(t, err)
	assert.Equal(t, "body of one", got.Task.Body)
}

func TestUpdateReplacesFields(t *testing.T) {
	c := browserClient(t)
	task := create(t, c, "one")

	resp, err := c.UpdateTask(context.Background(), &factoryv1.UpdateTaskRequest{
		Id: task.Id, Title: "two", Body: "new", Labels: []string{"x"},
		Priority: factoryv1.TaskPriority_TASK_PRIORITY_P0,
	})
	require.NoError(t, err)
	assert.Equal(t, "two", resp.Task.Title)
	assert.Equal(t, "new", resp.Task.Body)
	assert.Equal(t, factoryv1.TaskPriority_TASK_PRIORITY_P0, resp.Task.Priority)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_BACKLOG, resp.Task.Status)
}

func TestUpdateRecordsBodyEditorOnlyOnBodyChange(t *testing.T) {
	c := browserClient(t)
	task := create(t, c, "one")
	read := func() (string, string) {
		var by, via string
		require.NoError(t, testDB.QueryRow(context.Background(),
			`SELECT body_edited_by, body_edited_channel
			 FROM factory.tasks WHERE id = $1`, task.Id).Scan(&by, &via))
		return by, via
	}
	by, via := read()
	assert.Equal(t, "human:"+adminID, by)
	assert.Equal(t, "browser", via)

	_, err := testDB.Exec(context.Background(),
		`UPDATE factory.tasks SET body_edited_by = 'human:old',
		 body_edited_channel = 'other' WHERE id = $1`, task.Id)
	require.NoError(t, err)

	_, err = c.UpdateTask(context.Background(), &factoryv1.UpdateTaskRequest{
		Id: task.Id, Title: "renamed", Body: "body of one",
	})
	require.NoError(t, err)
	by, via = read()
	assert.Equal(t, "human:old", by, "title-only edit keeps the body editor")
	assert.Equal(t, "other", via)

	_, err = c.UpdateTask(context.Background(), &factoryv1.UpdateTaskRequest{
		Id: task.Id, Title: "renamed", Body: "changed",
	})
	require.NoError(t, err)
	by, via = read()
	assert.Equal(t, "human:"+adminID, by)
	assert.Equal(t, "browser", via)
}

func TestMoveBetweenColumns(t *testing.T) {
	c := browserClient(t)
	task := create(t, c, "one")

	for _, s := range []factoryv1.TaskStatus{
		factoryv1.TaskStatus_TASK_STATUS_IN_PROGRESS,
		factoryv1.TaskStatus_TASK_STATUS_IN_REVIEW,
		factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU,
		factoryv1.TaskStatus_TASK_STATUS_DONE,
		factoryv1.TaskStatus_TASK_STATUS_BACKLOG,
	} {
		moved, err := move(c, task.Id, s)
		require.NoError(t, err)
		assert.Equal(t, s, moved.Status)
	}
}

func TestMoveRejectsUnspecifiedStatus(t *testing.T) {
	c := browserClient(t)
	task := create(t, c, "one")

	_, err := move(c, task.Id, factoryv1.TaskStatus_TASK_STATUS_UNSPECIFIED)
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
}

func TestReadyAllowedForAdminInBrowser(t *testing.T) {
	c := browserClient(t)
	task := create(t, c, "one")

	moved, err := move(c, task.Id, factoryv1.TaskStatus_TASK_STATUS_READY)
	require.NoError(t, err)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_READY, moved.Status)
}

func TestReadyDeniedOverMCP(t *testing.T) {
	browser := browserClient(t)
	task := create(t, browser, "one")
	mcp := newClientKeepingBoard(t, true, constants.ChannelMCP)

	_, err := move(mcp, task.Id, factoryv1.TaskStatus_TASK_STATUS_READY)
	assert.Equal(t, connect.CodePermissionDenied, code(err))

	// Other columns stay open to the MCP channel.
	moved, err := move(mcp, task.Id, factoryv1.TaskStatus_TASK_STATUS_IN_PROGRESS)
	require.NoError(t, err)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_PROGRESS, moved.Status)
}

func TestReadyDeniedWithoutChannel(t *testing.T) {
	browser := browserClient(t)
	task := create(t, browser, "one")
	unknown := newClientKeepingBoard(t, true, "carrier-pigeon")

	_, err := move(unknown, task.Id, factoryv1.TaskStatus_TASK_STATUS_READY)
	assert.Equal(t, connect.CodePermissionDenied, code(err))
}

func TestNonAdminDeniedEverywhere(t *testing.T) {
	admin := browserClient(t)
	task := create(t, admin, "one")
	user := newClientKeepingBoard(t, false, constants.ChannelBrowser)
	ctx := context.Background()

	_, err := user.ListTasks(ctx, &factoryv1.ListTasksRequest{})
	assert.Equal(t, connect.CodePermissionDenied, code(err))
	_, err = user.GetTask(ctx, &factoryv1.GetTaskRequest{Id: task.Id})
	assert.Equal(t, connect.CodePermissionDenied, code(err))
	_, err = user.CreateTask(ctx, &factoryv1.CreateTaskRequest{Title: "x"})
	assert.Equal(t, connect.CodePermissionDenied, code(err))
	_, err = move(user, task.Id, factoryv1.TaskStatus_TASK_STATUS_DONE)
	assert.Equal(t, connect.CodePermissionDenied, code(err))
	_, err = user.UpdateTask(ctx, &factoryv1.UpdateTaskRequest{Id: task.Id, Title: "x"})
	assert.Equal(t, connect.CodePermissionDenied, code(err))
	_, err = user.AddComment(ctx, &factoryv1.AddCommentRequest{TaskId: task.Id, Body: "x"})
	assert.Equal(t, connect.CodePermissionDenied, code(err))
}

func TestCommentsAppearOnGetInOrder(t *testing.T) {
	c := browserClient(t)
	task := create(t, c, "one")
	ctx := context.Background()

	for _, body := range []string{"first", "  second  "} {
		_, err := c.AddComment(ctx, &factoryv1.AddCommentRequest{
			TaskId: task.Id, Body: body,
		})
		require.NoError(t, err)
	}

	got, err := c.GetTask(ctx, &factoryv1.GetTaskRequest{Id: task.Id})
	require.NoError(t, err)
	require.Len(t, got.Comments, 2)
	assert.Equal(t, "first", got.Comments[0].Body)
	assert.Equal(t, "second", got.Comments[1].Body)
	assert.Equal(t, "human:"+adminID, got.Comments[0].Author)
	assert.EqualValues(t, 2, got.Task.CommentCount)
}

func TestCommentValidation(t *testing.T) {
	c := browserClient(t)
	task := create(t, c, "one")
	ctx := context.Background()

	_, err := c.AddComment(ctx, &factoryv1.AddCommentRequest{TaskId: task.Id, Body: " "})
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
	_, err = c.AddComment(ctx, &factoryv1.AddCommentRequest{
		TaskId: task.Id, Body: strings.Repeat("a", 10001),
	})
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
}

func TestUnknownAndMalformedIDs(t *testing.T) {
	c := browserClient(t)
	ctx := context.Background()
	missing := "00000000-0000-0000-0000-000000000000"

	_, err := c.GetTask(ctx, &factoryv1.GetTaskRequest{Id: missing})
	assert.Equal(t, connect.CodeNotFound, code(err))
	_, err = c.UpdateTask(ctx, &factoryv1.UpdateTaskRequest{Id: missing, Title: "x"})
	assert.Equal(t, connect.CodeNotFound, code(err))
	_, err = move(c, missing, factoryv1.TaskStatus_TASK_STATUS_DONE)
	assert.Equal(t, connect.CodeNotFound, code(err))
	_, err = c.AddComment(ctx, &factoryv1.AddCommentRequest{TaskId: missing, Body: "x"})
	assert.Equal(t, connect.CodeNotFound, code(err))

	_, err = c.GetTask(ctx, &factoryv1.GetTaskRequest{Id: "nope"})
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
	_, err = c.UpdateTask(ctx, &factoryv1.UpdateTaskRequest{Id: "nope", Title: "x"})
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
	_, err = move(c, "nope", factoryv1.TaskStatus_TASK_STATUS_DONE)
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
	_, err = c.AddComment(ctx, &factoryv1.AddCommentRequest{TaskId: "nope", Body: "x"})
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
}
