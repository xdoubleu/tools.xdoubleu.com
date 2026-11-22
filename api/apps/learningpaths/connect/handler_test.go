package connect_test

// A nil service is safe: handlers return CodeUnauthenticated before touching
// it. The HTTP-level tests can't reach this, as mock auth always authenticates.

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"

	lpconnect "tools.xdoubleu.com/apps/learningpaths/connect"
	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
)

func TestHandlers_Unauthenticated(t *testing.T) {
	h := lpconnect.NewHandler(nil)
	th := lpconnect.NewTodoistHandler(nil)
	ctx := context.Background()

	calls := map[string]func() error{
		"list": func() error {
			_, err := h.ListLearningPaths(ctx, &learningpathsv1.ListLearningPathsRequest{})
			return err
		},
		"get": func() error {
			_, err := h.GetLearningPath(ctx, &learningpathsv1.GetLearningPathRequest{})
			return err
		},
		"create": func() error {
			_, err := h.CreateLearningPath(
				ctx, &learningpathsv1.CreateLearningPathRequest{},
			)
			return err
		},
		"update": func() error {
			_, err := h.UpdateLearningPath(
				ctx, &learningpathsv1.UpdateLearningPathRequest{},
			)
			return err
		},
		"delete": func() error {
			_, err := h.DeleteLearningPath(
				ctx, &learningpathsv1.DeleteLearningPathRequest{},
			)
			return err
		},
		"record": func() error {
			_, err := h.RecordItemProgress(
				ctx, &learningpathsv1.RecordItemProgressRequest{},
			)
			return err
		},
		"paused": func() error {
			_, err := h.SetLearningPathPaused(
				ctx, &learningpathsv1.SetLearningPathPausedRequest{},
			)
			return err
		},
		"progress": func() error {
			_, err := h.GetLearningPathProgress(
				ctx, &learningpathsv1.GetLearningPathProgressRequest{},
			)
			return err
		},
		"todoist connect": func() error {
			_, err := th.ConnectTodoist(ctx, &learningpathsv1.ConnectTodoistRequest{})
			return err
		},
		"todoist disconnect": func() error {
			_, err := th.DisconnectTodoist(
				ctx, &learningpathsv1.DisconnectTodoistRequest{},
			)
			return err
		},
		"todoist status": func() error {
			_, err := th.GetTodoistConnectionStatus(
				ctx, &learningpathsv1.GetTodoistConnectionStatusRequest{},
			)
			return err
		},
		"todoist sync state": func() error {
			_, err := th.GetTodoistSyncState(
				ctx, &learningpathsv1.GetTodoistSyncStateRequest{},
			)
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(call()))
		})
	}
}
