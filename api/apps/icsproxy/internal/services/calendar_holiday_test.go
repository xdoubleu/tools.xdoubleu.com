//nolint:testpackage // tests unexported helpers
package services

import (
	"strings"
	"testing"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/icsproxy/internal/models"
)

func makeVEvent(dtstart, dtend string) *ics.VEvent {
	data := buildICS(vevent("test-uid", "Test", dtstart, dtend))
	cal, _ := ics.ParseCalendar(strings.NewReader(string(data)))
	for _, c := range cal.Components {
		if ev, ok := c.(*ics.VEvent); ok {
			return ev
		}
	}
	return nil
}

func mustParseTime(raw string) time.Time {
	t, err := parseICSTime(raw)
	if err != nil {
		panic(err)
	}
	return t
}

// ── overlapsHoliday ──────────────────────────────────────────────────────────

func TestOverlapsHoliday(t *testing.T) {
	svc := newTestService()

	holiday := holidayWindow{
		start: mustParseTime("20240115T000000Z"),
		end:   mustParseTime("20240116T000000Z"),
	}

	tests := []struct {
		name     string
		evStart  string
		evEnd    string
		overlaps bool
	}{
		{"fully inside holiday", "20240115T090000Z", "20240115T100000Z", true},
		{"starts before ends during", "20240114T090000Z", "20240115T120000Z", true},
		{"starts during ends after", "20240115T120000Z", "20240117T090000Z", true},
		{"fully before holiday", "20240113T090000Z", "20240114T100000Z", false},
		{"fully after holiday", "20240117T090000Z", "20240117T100000Z", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ev := makeVEvent(tc.evStart, tc.evEnd)
			require.NotNil(t, ev)
			assert.Equal(t, tc.overlaps, svc.overlapsHoliday(ev, holiday))
		})
	}
}

// ── findHolidayWindows ───────────────────────────────────────────────────────

func TestFindHolidayWindows_NoHolidays(t *testing.T) {
	svc := newTestService()
	events := []models.EventInfo{
		{ //nolint:exhaustruct //only relevant fields needed
			UID:       "uid-1",
			SeriesKey: "Meeting",
			StartRaw:  "20240115T090000Z",
			EndRaw:    "20240115T100000Z",
		},
	}
	cfg := models.FilterConfig{ //nolint:exhaustruct //only relevant fields needed
		HolidayUIDs: []string{},
	}
	windows := svc.findHolidayWindows(events, cfg)
	assert.Empty(t, windows)
}

func TestFindHolidayWindows_WithHoliday(t *testing.T) {
	svc := newTestService()
	events := []models.EventInfo{
		{ //nolint:exhaustruct //only relevant fields needed
			UID:       "holiday-uid",
			SeriesKey: "Holiday",
			StartRaw:  "20240101T000000Z",
			EndRaw:    "20240102T000000Z",
		},
		{ //nolint:exhaustruct //only relevant fields needed
			UID:       "other-uid",
			SeriesKey: "Meeting",
			StartRaw:  "20240101T090000Z",
			EndRaw:    "20240101T100000Z",
		},
	}
	cfg := models.FilterConfig{ //nolint:exhaustruct //only relevant fields needed
		HolidayUIDs: []string{"holiday-uid"},
	}
	windows := svc.findHolidayWindows(events, cfg)
	assert.Len(t, windows, 1)
	assert.Equal(t, 2024, windows[0].start.Year())
}

func TestFindHolidayWindows_SkipsUnparseableTime(t *testing.T) {
	svc := newTestService()
	events := []models.EventInfo{
		{ //nolint:exhaustruct //only relevant fields needed
			UID:       "bad-uid",
			SeriesKey: "Holiday",
			StartRaw:  "not-a-time",
			EndRaw:    "also-bad",
		},
	}
	cfg := models.FilterConfig{ //nolint:exhaustruct //only relevant fields needed
		HolidayUIDs: []string{"bad-uid"},
	}
	windows := svc.findHolidayWindows(events, cfg)
	assert.Empty(t, windows)
}
