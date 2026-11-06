package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlausibleResetKeepsOnlyATimeInTheComingFifteenDays(t *testing.T) {
	now := time.Date(2030, 1, 2, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { v := now.Add(d); return &v }

	assert.Nil(t, plausibleReset(nil, now), "none said")
	assert.Nil(t, plausibleReset(at(-2*time.Minute), now), "already past")
	assert.Nil(t, plausibleReset(at(maxLimitAhead+time.Second), now), "too far ahead")

	for name, d := range map[string]time.Duration{
		"a moment ago (clock skew)": -30 * time.Second,
		"now":                       0,
		"hours ahead":               3 * time.Hour,
		"the furthest allowed":      maxLimitAhead,
	} {
		got := plausibleReset(at(d), now)
		require.NotNil(t, got, name)
		assert.True(t, got.Equal(now.Add(d)), name)
	}
}

func TestPlausibleResetIsKeptInUTC(t *testing.T) {
	now := time.Date(2030, 1, 2, 12, 0, 0, 0, time.UTC)
	local := now.Add(time.Hour).In(time.FixedZone("x", 5*3600))

	got := plausibleReset(&local, now)

	require.NotNil(t, got)
	assert.Equal(t, time.UTC, got.Location())
}

func TestLimitUntilNamesAKnownResetTimeAndIsEmptyOtherwise(t *testing.T) {
	assert.Empty(t, limitUntil(nil))

	at := time.Date(2030, 1, 2, 14, 30, 0, 0, time.FixedZone("x", 3600))
	assert.Equal(t, " (until 2030-01-02 13:30 UTC)", limitUntil(&at))
}
