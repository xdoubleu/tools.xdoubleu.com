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

// svc creates a CalendarService with nil repo/logger — safe for pure logic tests.
func newTestService() *CalendarService {
	//nolint:exhaustruct // repo/logger not needed for pure-logic tests
	return &CalendarService{}
}

// ── normalizeSummary ─────────────────────────────────────────────────────────

func TestNormalizeSummary(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"plain name unchanged", "Team Meeting", "Team Meeting"},
		{"strip date range", "Sprint Review 01/01/2024 - 31/01/2024", "Sprint Review"},
		{"strip version suffix", "Deploy Service - v1.2.3", "Deploy Service"},
		{"strip version no v", "Deploy Service - 1.2.3", "Deploy Service"},
		{"trim spaces", "  Meeting  ", "Meeting"},
		{"strip date slash format", "Event 1/1/24 - 31/1/24", "Event"},
		{"empty string", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, normalizeSummary(tc.input))
		})
	}
}

func TestMakeSeriesKey(t *testing.T) {
	assert.Equal(
		t,
		normalizeSummary("Team Meeting - v1.0"),
		makeSeriesKey("Team Meeting - v1.0"),
	)
}

// ── formatICSTime ────────────────────────────────────────────────────────────

func TestFormatICSTime(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"date only", "20240115", "15 Jan 2024"},
		{"datetime", "20240115T143000", "15 Jan 2024, 14:30"},
		{"VALUE=DATE prefix start", "DTSTART;VALUE=DATE:20240115", "15 Jan 2024"},
		{"VALUE=DATE prefix end", "DTEND;VALUE=DATE:20240115", "15 Jan 2024"},
		{"unparseable passthrough", "notadate", "notadate"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, formatICSTime(tc.input))
		})
	}
}

// ── parseICSTime ─────────────────────────────────────────────────────────────

func TestParseICSTime(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectErr   bool
		expectYear  int
		expectMonth time.Month
		expectDay   int
	}{
		{
			"UTC datetime Z",
			"20240115T143000Z",
			false,
			2024, time.January, 15,
		},
		{
			"datetime with offset",
			"20240115T143000+0100",
			false,
			2024, time.January, 15,
		},
		{
			"datetime no tz",
			"20240115T143000",
			false,
			2024, time.January, 15,
		},
		{
			"date only",
			"20240115",
			false,
			2024, time.January, 15,
		},
		{
			"VALUE=DATE prefix stripped",
			"DTSTART;VALUE=DATE:20240115",
			false,
			2024, time.January, 15,
		},
		{
			"invalid returns error",
			"not-a-time",
			true,
			0, 0, 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseICSTime(tc.input)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectYear, got.Year())
			assert.Equal(t, tc.expectMonth, got.Month())
			assert.Equal(t, tc.expectDay, got.Day())
		})
	}
}

// ── normalizeTZID ────────────────────────────────────────────────────────────

func TestNormalizeTZID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Romance Standard Time", "Europe/Brussels"},
		{"Central Europe Standard Time", "Europe/Berlin"},
		{"W. Europe Standard Time", "Europe/Amsterdam"},
		{"GMT Standard Time", "Europe/London"},
		{"Europe/Paris", "Europe/Paris"},
		{"America/New_York", "America/New_York"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, normalizeTZID(tc.input))
		})
	}
}

// ── ICS fixture helpers ──────────────────────────────────────────────────────

func buildICS(events ...string) []byte {
	var sb strings.Builder
	sb.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//Test//EN\r\n")
	for _, e := range events {
		sb.WriteString(e)
	}
	sb.WriteString("END:VCALENDAR\r\n")
	return []byte(sb.String())
}

func vevent(uid, summary, dtstart, dtend string, extras ...string) string {
	var sb strings.Builder
	sb.WriteString("BEGIN:VEVENT\r\n")
	sb.WriteString("UID:" + uid + "\r\n")
	sb.WriteString("SUMMARY:" + summary + "\r\n")
	sb.WriteString("DTSTART:" + dtstart + "\r\n")
	sb.WriteString("DTEND:" + dtend + "\r\n")
	for _, e := range extras {
		sb.WriteString(e + "\r\n")
	}
	sb.WriteString("END:VEVENT\r\n")
	return sb.String()
}

func recurringEvent(uid, summary, dtstart, dtend, rrule string) string {
	return vevent(uid, summary, dtstart, dtend, "RRULE:"+rrule)
}

// ── ExtractEvents ────────────────────────────────────────────────────────────

func TestExtractEvents_Basic(t *testing.T) {
	svc := newTestService()

	data := buildICS(
		vevent("uid-1", "Team Meeting", "20240115T090000Z", "20240115T100000Z"),
		vevent("uid-2", "Sprint Review", "20240116T090000Z", "20240116T100000Z"),
	)

	events, err := svc.ExtractEvents(data)
	require.NoError(t, err)
	assert.Len(t, events, 2)
}

func TestExtractEvents_DeduplicatesRecurring(t *testing.T) {
	svc := newTestService()

	data := buildICS(
		recurringEvent(
			"uid-1",
			"Daily Standup",
			"20240101T090000Z",
			"20240101T091500Z",
			"FREQ=DAILY",
		),
		vevent("uid-2", "Daily Standup", "20240101T090000Z", "20240101T091500Z",
			"RECURRENCE-ID:20240102T090000Z"),
	)

	events, err := svc.ExtractEvents(data)
	require.NoError(t, err)
	// Modified instance (RECURRENCE-ID) should be excluded from grouped view
	assert.Len(t, events, 1)
	assert.Equal(t, "Daily Standup", events[0].Summary)
}

func TestExtractEvents_NormalizesSeriesKey(t *testing.T) {
	svc := newTestService()

	data := buildICS(
		vevent("uid-1", "Sprint Review 01/01/2024 - 31/01/2024",
			"20240101T090000Z", "20240101T100000Z"),
		vevent("uid-2", "Sprint Review 01/02/2024 - 28/02/2024",
			"20240201T090000Z", "20240201T100000Z"),
	)

	events, err := svc.ExtractEvents(data)
	require.NoError(t, err)
	// Both have the same series key "Sprint Review" → only one in grouped
	assert.Len(t, events, 1)
}

func TestExtractEvents_InvalidICS(t *testing.T) {
	svc := newTestService()
	// ics.ParseCalendar is lenient; verify it does not panic
	_, _ = svc.ExtractEvents([]byte("not valid ics"))
	t.Log("no panic on invalid ICS")
}

// ── isExplicitlyHidden ───────────────────────────────────────────────────────

func TestIsExplicitlyHidden(t *testing.T) {
	svc := newTestService()

	cfg := models.FilterConfig{ //nolint:exhaustruct // only needed fields
		HideEventUIDs: []string{"uid-a", "uid-b"},
	}

	assert.True(t, svc.isExplicitlyHidden("uid-a", cfg))
	assert.True(t, svc.isExplicitlyHidden("uid-b", cfg))
	assert.False(t, svc.isExplicitlyHidden("uid-c", cfg))
	assert.False(t, svc.isExplicitlyHidden("", cfg))
}

// ── overlapsHoliday ──────────────────────────────────────────────────────────

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

func mustParseTime(raw string) time.Time {
	t, err := parseICSTime(raw)
	if err != nil {
		panic(err)
	}
	return t
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

// ── ApplyFilter ──────────────────────────────────────────────────────────────

func TestApplyFilter_HidesExplicitUID(t *testing.T) {
	svc := newTestService()

	data := buildICS(
		vevent("hide-me", "Secret", "20240115T090000Z", "20240115T100000Z"),
		vevent("keep-me", "Public", "20240115T090000Z", "20240115T100000Z"),
	)
	cfg := models.FilterConfig{ //nolint:exhaustruct // only needed fields
		HideEventUIDs: []string{"hide-me"},
	}

	out, err := svc.ApplyFilter(data, cfg)
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

	out, err := svc.ApplyFilter(data, cfg)
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

	out, err := svc.ApplyFilter(data, cfg)
	require.NoError(t, err)
	outStr := string(out)
	assert.Contains(t, outStr, "Holiday")
	assert.NotContains(t, outStr, "Work")
}

func TestApplyFilter_InvalidICS(t *testing.T) {
	svc := newTestService()
	//nolint:exhaustruct // only needed fields
	_, err := svc.ApplyFilter([]byte("garbage"), models.FilterConfig{})
	require.Error(t, err)
}

func TestApplyFilter_EmptyConfig(t *testing.T) {
	svc := newTestService()

	data := buildICS(
		vevent("uid-1", "Event One", "20240115T090000Z", "20240115T100000Z"),
		vevent("uid-2", "Event Two", "20240116T090000Z", "20240116T100000Z"),
	)
	//nolint:exhaustruct // intentionally empty config
	out, err := svc.ApplyFilter(data, models.FilterConfig{})
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

	out, err := svc.ApplyFilter(data, cfg)
	require.NoError(t, err)
	outStr := string(out)
	assert.Contains(t, outStr, "EXDATE")
	assert.Contains(t, outStr, "Daily Standup")
}

// ── parseICSTimeWithTZID ─────────────────────────────────────────────────────

func TestParseICSTimeWithTZID_WithTZID(t *testing.T) {
	p := &ics.IANAProperty{
		BaseProperty: ics.BaseProperty{
			IANAToken: "DTSTART",
			Value:     "20240115T090000",
			ICalParameters: map[string][]string{
				"TZID": {"Europe/Brussels"},
			},
		},
	}

	got, err := parseICSTimeWithTZID(p)
	require.NoError(t, err)
	assert.Equal(t, 2024, got.Year())
	assert.Equal(t, time.January, got.Month())
	assert.Equal(t, 15, got.Day())
	assert.Equal(t, 9, got.Hour())
}

func TestParseICSTimeWithTZID_NoTZID_FallsBackToParseICSTime(t *testing.T) {
	p := &ics.IANAProperty{
		BaseProperty: ics.BaseProperty{
			IANAToken:      "DTSTART",
			Value:          "20240115T090000Z",
			ICalParameters: map[string][]string{},
		},
	}

	got, err := parseICSTimeWithTZID(p)
	require.NoError(t, err)
	assert.Equal(t, 2024, got.Year())
}

func TestParseICSTimeWithTZID_InvalidTZID(t *testing.T) {
	p := &ics.IANAProperty{
		BaseProperty: ics.BaseProperty{
			IANAToken: "DTSTART",
			Value:     "20240115T090000",
			ICalParameters: map[string][]string{
				"TZID": {"Invalid/NotATZ"},
			},
		},
	}

	// Falls back to parseICSTime when location load fails
	got, err := parseICSTimeWithTZID(p)
	require.NoError(t, err)
	assert.Equal(t, 2024, got.Year())
}
