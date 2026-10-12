package feeds

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/feeds/internal/models"
	"tools.xdoubleu.com/internal/consumption"
)

// engaged builds an item with only what itemSignal reads.
func engaged(read bool, bookmarked bool, pct int) models.EngagedItem {
	var readAt *time.Time
	if read {
		now := time.Now()
		readAt = &now
	}
	//nolint:exhaustruct // only what itemSignal reads
	return models.EngagedItem{ReadAt: readAt, Bookmarked: bookmarked, ReadProgressPct: pct}
}

func TestItemSignal(t *testing.T) {
	tests := []struct {
		name   string
		item   models.EngagedItem
		signal consumption.Signal
		reason string
	}{
		{
			"bookmarked unread",
			engaged(false, true, 0),
			consumption.StrongPositive,
			"bookmarked",
		},
		{"dismissed", engaged(false, false, 0), consumption.Negative, "dismissed"},
		{
			"read in full",
			engaged(true, false, readInFullPct),
			consumption.Positive,
			"read in full",
		},
		{"just short", engaged(true, false, readInFullPct-1), consumption.Neutral, "opened"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signal, reason := itemSignal(tt.item)
			assert.Equal(t, tt.signal, signal)
			assert.Equal(t, tt.reason, reason)
		})
	}
}
