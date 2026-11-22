package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/models"
)

func TestToggleTag(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c"}, toggleTag([]string{"a", "b"}, "c", true))
	assert.Equal(t, []string{"a", "c"}, toggleTag([]string{"a", "b", "c"}, "b", false))
	assert.Equal(t, []string{"b", "a"}, toggleTag([]string{"a", "b"}, "a", true))
	assert.Equal(t, []string{"a", "b"}, toggleTag([]string{"a", "b"}, "z", false))
	assert.Equal(t, []string{"tag"}, toggleTag(nil, "tag", true))
}

func TestParseRating(t *testing.T) {
	assert.Nil(t, ParseRating(""))
	assert.Nil(t, ParseRating("0"))
	assert.Nil(t, ParseRating("6"))
	assert.Nil(t, ParseRating("x"))
	got := ParseRating("4")
	require.NotNil(t, got)
	assert.Equal(t, int16(4), *got)
}

func TestFinishedAtFor(t *testing.T) {
	earlier := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

	assert.Nil(t, finishedAtFor(nil, models.StatusReading))
	assert.Len(t, finishedAtFor(nil, models.StatusRead), 1)

	//nolint:exhaustruct // only fields under test
	was := &models.UserBook{Status: models.StatusReading, FinishedAt: []time.Time{earlier}}
	got := finishedAtFor(was, models.StatusRead)
	require.Len(t, got, 2)
	assert.Equal(t, earlier, got[0])

	//nolint:exhaustruct // only fields under test
	already := &models.UserBook{
		Status: models.StatusRead, FinishedAt: []time.Time{earlier},
	}
	assert.Equal(t, []time.Time{earlier}, finishedAtFor(already, models.StatusRead))
}

func TestParseProgressRange(t *testing.T) {
	start, end := ParseProgressRange("2024-01-02", "2024-03-04")
	assert.Equal(t, "2024-01-02", start.Format(models.ProgressDateFormat))
	assert.Equal(t, "2024-03-04", end.Format(models.ProgressDateFormat))

	start, end = ParseProgressRange("", "bad")
	assert.InDelta(t, 365*24, end.Sub(start).Hours(), 48)
}
