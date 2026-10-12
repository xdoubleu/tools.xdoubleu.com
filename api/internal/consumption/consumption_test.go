package consumption_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/consumption"
)

func TestRatingSignal(t *testing.T) {
	for stars, want := range map[int]consumption.Signal{
		0: consumption.StrongNegative,
		1: consumption.StrongNegative,
		2: consumption.Negative,
		3: consumption.Neutral,
		4: consumption.Positive,
		5: consumption.StrongPositive,
		6: consumption.StrongPositive,
	} {
		assert.Equal(t, want, consumption.RatingSignal(stars), "%d stars", stars)
	}
}

func TestRatingReason(t *testing.T) {
	assert.Equal(t, "rated 4★", consumption.RatingReason(4))
}

func TestLatest(t *testing.T) {
	early := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	late := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	assert.True(t, consumption.Latest(nil).IsZero())
	assert.Equal(t, late, consumption.Latest([]*time.Time{&early, nil, &late}))
}

func TestDomains(t *testing.T) {
	assert.Len(t, consumption.Domains(), 6)
	assert.Equal(t, consumption.DomainBooks, consumption.Domains()[0])
}
