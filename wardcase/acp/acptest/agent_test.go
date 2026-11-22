package acptest_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/acp"
	"tools.xdoubleu.com/wardcase/acp/acptest"
)

func TestTheScriptedAgentAnswersAPromptAndRecordsTheAnswerToItsPermissionRequest(t *testing.T) {
	toAgentR, toAgentW := io.Pipe()
	toClientR, toClientW := io.Pipe()
	agent := acptest.New()
	go agent.Serve(context.Background(), toAgentR, toClientW)
	client := acp.NewClient(toClientR, toAgentW, nil)
	go func() { _ = client.Serve(context.Background()) }()
	defer toAgentW.Close()

	ctx := context.Background()
	require.NoError(t, client.Initialize(ctx))
	id, err := client.NewSession(ctx, "/work")
	require.NoError(t, err)
	reason, err := client.Prompt(ctx, id, "hi")
	require.NoError(t, err)
	assert.Equal(t, "end_turn", reason)
	assert.Equal(t, []string{"hi"}, agent.Prompts())
	assert.Len(t, agent.Outcomes(), 1)

	go func() { _, _ = client.Prompt(ctx, id, "wait") }()
	require.Eventually(t, func() bool { return len(agent.Prompts()) == 2 },
		time.Second, time.Millisecond)
	require.NoError(t, client.Cancel(id))
}
