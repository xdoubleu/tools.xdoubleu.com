//nolint:testpackage // tests unexported helpers
package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tools.xdoubleu.com/apps/icsproxy/internal/models"
)

func TestApplyFilter_HidesExplicitUID(t *testing.T) {
	svc := newTestService()

	data := buildICS(
		vevent("hide-me", "Secret", "20240115T090000Z", "20240115T100000Z"),
		vevent("keep-me", "Public", "20240115T090000Z", "20240115T100000Z"),
	)
	cfg := models.FilterConfig{ //nolint:exhaustruct // only needed fields
		HideEventUIDs: []string{"hide-me"},
	}

	out, err := svc.ApplyFilter(context.Background(), data, cfg)
	require.NoError(t, err)
	outStr := string(out)
	assert.NotContains(t, outStr, "Secret")
	assert.Contains(t, outStr, "Public")
}

func TestApplyFilter_HidesSeries(t *testing.T) {
	svc := newTestService()

	data := buildICS(
		recurringEvent("uid-1", "Daily Standup",
			"20240115T090000Z", "20240115T091500Z", "FREQ=DAILY"),
		vevent("uid-2", "Weekly Review", "20240115T100000Z", "20240115T110000Z"),
	)
	cfg := models.FilterConfig{ //nolint:exhaustruct // only needed fields
		HideSeries: map[string]bool{"Daily Standup": true},
	}

	out, err := svc.ApplyFilter(context.Background(), data, cfg)
	require.NoError(t, err)
	outStr := string(out)
	assert.NotContains(t, outStr, "Daily Standup")
	assert.Contains(t, outStr, "Weekly Review")
}

func TestApplyFilter_PreservesHolidayEvent(t *testing.T) {
	svc := newTestService()

	// A holiday + an event that overlaps it — holiday must be preserved
	data := buildICS(
		vevent("holiday-uid", "Holiday", "20240115T000000Z", "20240116T000000Z"),
		vevent("overlap-uid", "Work", "20240115T090000Z", "20240115T100000Z"),
	)
	cfg := models.FilterConfig{ //nolint:exhaustruct // only needed fields
		HolidayUIDs: []string{"holiday-uid"},
	}

	out, err := svc.ApplyFilter(context.Background(), data, cfg)
	require.NoError(t, err)
	outStr := string(out)
	assert.Contains(t, outStr, "Holiday")
	assert.NotContains(t, outStr, "Work")
}

func TestApplyFilter_InvalidICS(t *testing.T) {
	svc := newTestService()
	//nolint:exhaustruct // only needed fields
	_, err := svc.ApplyFilter(
		context.Background(),
		[]byte("garbage"),
		models.FilterConfig{},
	)
	require.Error(t, err)
}

func TestApplyFilter_EmptyConfig(t *testing.T) {
	svc := newTestService()

	data := buildICS(
		vevent("uid-1", "Event One", "20240115T090000Z", "20240115T100000Z"),
		vevent("uid-2", "Event Two", "20240116T090000Z", "20240116T100000Z"),
	)
	//nolint:exhaustruct // intentionally empty config
	out, err := svc.ApplyFilter(context.Background(), data, models.FilterConfig{})
	require.NoError(t, err)
	outStr := string(out)
	assert.Contains(t, outStr, "Event One")
	assert.Contains(t, outStr, "Event Two")
}

func TestApplyFilter_RecurringWithHolidayWindow(t *testing.T) {
	svc := newTestService()

	// Recurring event + a holiday day → EXDATE should be added
	data := buildICS(
		vevent("holiday-uid", "Public Holiday", "20240115T000000Z", "20240116T000000Z"),
		recurringEvent("standup-uid", "Daily Standup",
			"20240101T090000Z", "20240101T091500Z", "FREQ=DAILY"),
	)
	cfg := models.FilterConfig{ //nolint:exhaustruct // only needed fields
		HolidayUIDs: []string{"holiday-uid"},
	}

	out, err := svc.ApplyFilter(context.Background(), data, cfg)
	require.NoError(t, err)
	outStr := string(out)
	assert.Contains(t, outStr, "EXDATE")
	assert.Contains(t, outStr, "Daily Standup")
}
