package factory_test

import (
	"context"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/factory"
	"tools.xdoubleu.com/internal/platform/factory/messyseed"
	"tools.xdoubleu.com/internal/platform/githubapp"
)

// The TestMessyReads tests (`make test/reads`) call every read of the factory
// on seedMessyBoard's data and fail on any error. A query that is fine on one
// row per relation, like a scalar subquery over a link table, breaks here.

func messyEnv(t *testing.T) (jobEnv, messyseed.Board, *digestFeed) {
	t.Helper()
	feed := &digestFeed{value: atomic.Value{}, fail: atomic.Bool{}}
	feed.set(uuid.NewString())
	env, gh := githubEnv(t, factory.WithImageDigestSource(feed.read))
	board := messyseed.Seed(t, testDB)
	for _, n := range []int{1, 4} {
		gh.answer(n, githubapp.PullRequestStatus{
			State: githubapp.StateOpen, CI: githubapp.CIPassing, Review: "",
			Mergeable: "", MergeSHA: "", HeadSHA: "",
		}, nil)
	}
	return env, board, feed
}

func TestMessyReadsTheBoard(t *testing.T) {
	env, board, _ := messyEnv(t)
	ctx := context.Background()

	list, err := env.tasks.ListTasks(ctx, &factoryv1.ListTasksRequest{})
	require.NoError(t, err)
	assert.Len(t, list.Tasks, len(board.TaskIDs))

	for _, id := range board.TaskIDs {
		_, err = env.tasks.GetTask(ctx, &factoryv1.GetTaskRequest{Id: id})
		require.NoError(t, err, id)
	}
	for _, id := range board.JobIDs {
		_, err = env.tasks.ListJobEvents(ctx, &factoryv1.ListJobEventsRequest{
			JobId: id, AfterSeq: 1,
		})
		require.NoError(t, err, id)
	}

	_, err = env.tasks.ListRoutines(ctx, &factoryv1.ListRoutinesRequest{})
	require.NoError(t, err)
	_, err = env.admin.ListRunners(ctx, &factoryv1.ListRunnersRequest{})
	require.NoError(t, err)
	repos := factoryv1connect.NewRepoServiceClient(
		connect.NewClient(newTransport(t, env.app)),
	)
	_, err = repos.ListRepos(ctx, &factoryv1.ListReposRequest{})
	require.NoError(t, err)
	_, err = env.finished(t, board.TaskIDs...)
	require.NoError(t, err)
}

func TestMessyReadsTheTodayBlock(t *testing.T) {
	env, _, _ := messyEnv(t)

	got, err := env.app.ListAttentionTasks(context.Background(), 10)
	require.NoError(t, err)
	assert.NotEmpty(t, got)
}

func TestMessyReadsTheMCPTools(t *testing.T) {
	env, board, _ := messyEnv(t)
	session := mcpSessionOn(t, env.app, asUser(models.RoleAgent))

	_, err := callTool(t, session, "factory_list_repos", map[string]any{})
	require.NoError(t, err)
	_, err = callTool(t, session, "factory_list_tasks", map[string]any{})
	require.NoError(t, err)
	_, err = callTool(t, session, "factory_list_tasks",
		map[string]any{"query": "ünï", "label": "BUG", "status": "ready"})
	require.NoError(t, err)
	for _, id := range board.TaskIDs {
		_, err = callTool(t, session, "factory_get_task", map[string]any{"id": id})
		require.NoError(t, err, id)
	}
}

func TestMessyReadsTheBackgroundJobs(t *testing.T) {
	env, _, feed := messyEnv(t)
	ctx := context.Background()

	// The first digest is recorded; the next resumes the parked task.
	require.NoError(t, env.app.ResumeParkedNow(ctx))
	feed.set(uuid.NewString())
	require.NoError(t, env.app.ResumeParkedNow(ctx))

	require.NoError(t, env.app.PollPullRequestsNow(ctx))
	require.NoError(t, env.app.CollectSignalsNow(ctx))
	require.NoError(t, env.app.CheckRunnerNow(ctx))
	require.NoError(t, env.app.EnqueueRoutinesNow(ctx))
	require.NoError(t, env.app.PruneEventsNow(ctx))

	// The board still lists after the jobs wrote to it.
	_, err := env.tasks.ListTasks(ctx, &factoryv1.ListTasksRequest{})
	require.NoError(t, err)
}
