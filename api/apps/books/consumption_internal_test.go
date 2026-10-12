package books

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/internal/consumption"
)

// userBook builds a UserBook with only what bookSignal reads.
func userBook(
	status string,
	rating *int16,
	tags []string,
	finished []time.Time,
) models.UserBook {
	//nolint:exhaustruct // only what bookSignal reads
	return models.UserBook{
		Status: status, Rating: rating, Tags: tags, FinishedAt: finished,
	}
}

func TestBookSignal(t *testing.T) {
	two := int16(2)
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		ub     models.UserBook
		signal consumption.Signal
		reason string
		ok     bool
	}{
		{"to read", userBook(models.StatusToRead, nil, nil, nil), 0, "", false},
		{"reading", userBook(models.StatusReading, nil, nil, nil), 0, "", false},
		{
			"rating wins over dropped",
			userBook(models.StatusDropped, &two, nil, nil),
			consumption.Negative, "rated 2★", true,
		},
		{
			"dropped", userBook(models.StatusDropped, nil, nil, nil),
			consumption.Negative, "abandoned", true,
		},
		{
			"favourite",
			userBook(models.StatusRead, nil, []string{models.TagFavourite}, nil),
			consumption.StrongPositive, "favourite", true,
		},
		{
			"re-read",
			userBook(models.StatusRead, nil, nil, []time.Time{day, day}),
			consumption.StrongPositive, "re-read", true,
		},
		{
			"finished", userBook(models.StatusRead, nil, nil, nil),
			consumption.Positive, "finished", true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signal, reason, ok := bookSignal(tt.ub)
			assert.Equal(t, tt.signal, signal)
			assert.Equal(t, tt.reason, reason)
			assert.Equal(t, tt.ok, ok)
		})
	}
}
