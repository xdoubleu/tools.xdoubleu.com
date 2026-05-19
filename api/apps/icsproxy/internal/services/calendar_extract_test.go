//nolint:testpackage // tests unexported helpers
package services

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/icsproxy/internal/models"
)

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

	events, err := svc.ExtractEvents(context.Background(), data)
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

	events, err := svc.ExtractEvents(context.Background(), data)
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

	events, err := svc.ExtractEvents(context.Background(), data)
	require.NoError(t, err)
	// Both have the same series key "Sprint Review" → only one in grouped
	assert.Len(t, events, 1)
}

func TestExtractEvents_InvalidICS(t *testing.T) {
	svc := newTestService()
	// ics.ParseCalendar is lenient; verify it does not panic
	_, _ = svc.ExtractEvents(context.Background(), []byte("not valid ics"))
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
