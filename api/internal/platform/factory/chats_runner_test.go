package factory_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
	"tools.xdoubleu.com/internal/platform/factory/internal/repositories"
)

func TestARunnerClaimsItsChatAndEndsItWithAReport(t *testing.T) {
	env := browserJobEnv(t)
	ctx := context.Background()
	var runnerID uuid.UUID
	require.NoError(t, testDB.QueryRow(ctx,
		`SELECT id FROM factory.runners WHERE name = 'Home Mac'`).Scan(&runnerID))
	var repoID uuid.UUID
	repoID, err := uuid.Parse(seededRepoID(t))
	require.NoError(t, err)
	var in models.ChatSession
	in.RepoID, in.RunnerID, in.Harness = repoID, runnerID, models.HarnessOpenCode
	in.Branch = "agent/chat-abcd1234"
	chat, err := repositories.New(testDB).Chats.Start(ctx, in, "human:u")
	require.NoError(t, err)

	claimed := env.claim(t)

	require.NotNil(t, claimed)
	assert.Equal(t, chat.ID.String(), claimed.ChatId)
	assert.Equal(t, chat.JobID.String(), claimed.Id)
	assert.Equal(t, "agent/chat-abcd1234", claimed.Branch)
	assert.Empty(t, claimed.TaskId)
	assert.Empty(t, claimed.Prompt)

	_, err = env.runner.EndChat(bearer(env.token), &factoryv1.EndChatRequest{
		ChatId: chat.ID.String(), Report: "2 files left",
	})
	require.NoError(t, err)
	var state, report string
	require.NoError(t, testDB.QueryRow(ctx,
		`SELECT state, report FROM factory.chat_sessions WHERE id = $1`, chat.ID,
	).Scan(&state, &report))
	assert.Equal(t, "stopped", state)
	assert.Equal(t, "2 files left", report)

	_, err = env.runner.EndChat(bearer(env.token), &factoryv1.EndChatRequest{
		ChatId: "not-a-uuid",
	})
	require.Error(t, err)
}
