package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

func TestStatusValid(t *testing.T) {
	for _, s := range []models.Status{
		models.StatusBacklog, models.StatusReady, models.StatusInProgress,
		models.StatusInReview, models.StatusNeedsYou, models.StatusDone,
		models.StatusTesting,
	} {
		assert.True(t, s.Valid(), s)
	}
	assert.False(t, models.Status("").Valid())
	assert.False(t, models.Status("archived").Valid())
}

func TestPriorityValid(t *testing.T) {
	for _, p := range []models.Priority{
		models.PriorityP0, models.PriorityP1, models.PriorityP2,
	} {
		assert.True(t, p.Valid(), p)
	}
	assert.False(t, models.Priority("").Valid())
	assert.False(t, models.PriorityInvalid.Valid())
}

func TestActorString(t *testing.T) {
	a := models.Actor{
		Kind:    models.ActorHuman,
		ID:      "u1",
		Browser: true,
		Admin:   true,
		Via:     "",
	}
	assert.Equal(t, "human:u1", a.String())
}

func TestActorChannel(t *testing.T) {
	assert.Equal(
		t,
		"browser",
		models.Actor{
			Kind:    "",
			ID:      "",
			Browser: true,
			Admin:   false,
			Via:     "",
		}.Channel(),
	)
	assert.Equal(
		t,
		"other",
		models.Actor{
			Kind:    "",
			ID:      "",
			Browser: false,
			Admin:   false,
			Via:     "",
		}.Channel(),
	)
}

func TestIsBrowserAdminNeedsHumanAdminInBrowser(t *testing.T) {
	ok := models.Actor{
		Kind:    models.ActorHuman,
		ID:      "u",
		Browser: true,
		Admin:   true,
		Via:     "",
	}
	assert.True(t, ok.IsBrowserAdmin())

	noBrowser := ok
	noBrowser.Browser = false
	assert.False(t, noBrowser.IsBrowserAdmin())

	noAdmin := ok
	noAdmin.Admin = false
	assert.False(t, noAdmin.IsBrowserAdmin())

	otherKind := ok
	otherKind.Kind = "agent"
	assert.False(t, otherKind.IsBrowserAdmin())
}

func TestReadyAndRoutinesFollowThePermissionNotTheChannel(t *testing.T) {
	adminOverMCP := models.Actor{
		Kind:    models.ActorHuman,
		ID:      "u",
		Browser: false,
		Admin:   true,
		Via:     "",
	}
	assert.True(t, adminOverMCP.CanMoveToReady())
	assert.True(t, adminOverMCP.CanManageRoutines())
	assert.False(t, adminOverMCP.IsBrowserAdmin())

	agent := models.Actor{
		Kind:    models.ActorAgent,
		ID:      "a",
		Browser: false,
		Admin:   false,
		Via:     "",
	}
	assert.True(t, agent.CanMoveToReady())
	assert.True(t, agent.CanManageRoutines())

	person := adminOverMCP
	person.Admin = false
	assert.False(t, person.CanMoveToReady())
	assert.False(t, person.CanManageRoutines())
}

func TestHarnessValid(t *testing.T) {
	assert.True(t, models.HarnessClaudeCode.Valid())
	assert.True(t, models.HarnessOpenCode.Valid())
	assert.False(t, models.Harness("").Valid())
	assert.False(t, models.HarnessInvalid.Valid())
}

func TestTaskRefRoundTrips(t *testing.T) {
	assert.Equal(t, "T-142", models.TaskRef(142))
	for _, ref := range []string{"T-142", "t-142", "t142", " 142 "} {
		n, ok := models.ParseTaskRef(ref)
		assert.True(t, ok, ref)
		assert.EqualValues(t, 142, n, ref)
	}
}

func TestParseTaskRefRejectsNonNumbers(t *testing.T) {
	for _, ref := range []string{"", "T-", "T-0", "T--1", "abc", "211de0a1"} {
		_, ok := models.ParseTaskRef(ref)
		assert.False(t, ok, ref)
	}
}

func TestHasLabel(t *testing.T) {
	assert.True(t, models.HasLabel([]string{"bug", "Spike"}, models.LabelSpike))
	assert.False(t, models.HasLabel([]string{"bug"}, models.LabelSpike))
	assert.False(t, models.HasLabel(nil, models.LabelSpike))
}
