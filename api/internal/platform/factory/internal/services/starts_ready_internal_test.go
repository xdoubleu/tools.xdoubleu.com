package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

func TestStartsReady(t *testing.T) {
	actor := func(kind models.ActorKind, id string) models.Actor {
		return models.Actor{Kind: kind, ID: id, Browser: false, Admin: false, Via: ""}
	}
	agent := actor(models.ActorAgent, "agent-1")
	runner := actor(models.ActorAgent, "runner:abc")
	in := func(labels ...string) TaskInput {
		return input("A title", "A body.", models.PriorityP2, labels)
	}
	assert.True(t, startsReady(agent, in("bug")))
	assert.True(t, startsReady(agent, in("routine-found")))
	assert.True(t, startsReady(runner, in(SandboxLabel, "bug")))
	assert.True(t, startsReady(runner, in(SandboxLabel, "factory-retro")))
	assert.False(t, startsReady(agent, in("feature")))
	assert.False(t, startsReady(runner, in("bug")), "no sandbox label")
	assert.False(t, startsReady(runner, in(SandboxLabel)), "neither bug nor retro")
	assert.False(t, startsReady(actor(models.ActorHuman, "u"), in("bug")))
}

func TestStartColumn(t *testing.T) {
	agent := models.Actor{
		Kind: models.ActorAgent, ID: "a", Browser: false, Admin: false, Via: "",
	}
	human := models.Actor{
		Kind: models.ActorHuman, ID: "u", Browser: true, Admin: false, Via: "",
	}
	with := func(status models.Status, labels ...string) TaskInput {
		in := input("A title", "A body.", models.PriorityP2, labels)
		in.Status = status
		return in
	}
	for _, c := range []struct {
		actor models.Actor
		in    TaskInput
		want  models.Status
	}{
		{agent, with(""), models.StatusBacklog},
		{agent, with("", "bug"), models.StatusReady},
		{agent, with(models.StatusBacklog, "bug"), models.StatusBacklog},
		{agent, with(models.StatusReady), models.StatusReady},
		{human, with(models.StatusBacklog), models.StatusBacklog},
	} {
		got, err := startColumn(c.actor, c.in)
		assert.NoError(t, err)
		assert.Equal(t, c.want, got)
	}
	for _, in := range []TaskInput{
		with(models.StatusDone), with(models.StatusTesting), with("nope"),
		with(models.StatusReady, models.LabelSpike),
	} {
		_, err := startColumn(agent, in)
		assert.Error(t, err, in.Status)
	}
	_, err := startColumn(human, with(models.StatusReady))
	assert.Error(t, err, "a non-admin human can't approve")
}
