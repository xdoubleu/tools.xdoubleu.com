//nolint:testpackage // tests unexported helpers
package services

import (
	"testing"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestService creates a CalendarService with nil repo/logger — safe for
// pure logic tests.
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
