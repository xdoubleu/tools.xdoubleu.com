package models_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/factory/internal/models"
)

func at(t time.Time) *time.Time { return &t }

func runner(paired, seen, revoked *time.Time) models.Runner {
	return models.Runner{
		ID: uuid.Nil, Name: "r", CreatedBy: "human:x", CreatedAt: time.Time{},
		PairingExpiresAt: nil, PairedAt: paired, LastSeen: seen, Version: "1",
		RevokedAt: revoked,
	}
}

func TestRunnerIsOnlineWhenPairedActiveAndRecentlySeen(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	paired := at(now.Add(-time.Hour))

	assert.True(t, runner(paired, at(now.Add(-10*time.Second)), nil).Online(now))
	assert.True(
		t,
		runner(paired, at(now.Add(-models.OnlineWindow+time.Second)), nil).Online(now),
	)

	assert.False(t, runner(paired, at(now.Add(-models.OnlineWindow)), nil).Online(now),
		"the window is exclusive")
	assert.False(t, runner(paired, at(now.Add(-time.Hour)), nil).Online(now))
	assert.False(t, runner(paired, nil, nil).Online(now), "never heard from")
	assert.False(t, runner(nil, at(now), nil).Online(now), "not paired yet")
	assert.False(t,
		runner(paired, at(now), at(now.Add(-time.Minute))).Online(now), "revoked")
}
