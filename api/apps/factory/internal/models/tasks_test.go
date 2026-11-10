package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/factory/internal/models"
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
	a := models.Actor{Kind: models.ActorHuman, ID: "u1", Browser: true, Admin: true}
	assert.Equal(t, "human:u1", a.String())
}

func TestActorChannel(t *testing.T) {
	assert.Equal(
		t,
		"browser",
		models.Actor{Kind: "", ID: "", Browser: true, Admin: false}.Channel(),
	)
	assert.Equal(
		t,
		"other",
		models.Actor{Kind: "", ID: "", Browser: false, Admin: false}.Channel(),
	)
}

func TestCanMoveToReadyNeedsHumanAdminInBrowser(t *testing.T) {
	ok := models.Actor{Kind: models.ActorHuman, ID: "u", Browser: true, Admin: true}
	assert.True(t, ok.CanMoveToReady())

	noBrowser := ok
	noBrowser.Browser = false
	assert.False(t, noBrowser.CanMoveToReady())

	noAdmin := ok
	noAdmin.Admin = false
	assert.False(t, noAdmin.CanMoveToReady())

	otherKind := ok
	otherKind.Kind = "agent"
	assert.False(t, otherKind.CanMoveToReady())
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
