//nolint:testpackage // tests unexported helpers
package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tools.xdoubleu.com/apps/todos/internal/models"
)

// ── subtask input parsing (parseQuickInput reuse) ─────────────────────────────

func TestSubtaskInput_ParsesPriority(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	title, dto := parseQuickInput("Fix tests p1", nil, now)
	assert.Equal(t, "Fix tests", title)
	assert.Equal(t, 1, dto.Priority)
}

func TestSubtaskInput_ParsesLabelAndDue(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	title, dto := parseQuickInput("Write docs @docs tomorrow", nil, now)
	assert.Equal(t, "Write docs", title)
	assert.Equal(t, "docs", dto.Label)
	assert.Equal(t, "2026-05-09", dto.DueDate)
}

func TestSubtaskInput_ParsesDeadline(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	title, dto := parseQuickInput("Ship feature !today", nil, now)
	assert.Equal(t, "Ship feature", title)
	assert.Equal(t, "2026-05-08", dto.Deadline)
}

// ── nextRecurringDue ──────────────────────────────────────────────────────────

func TestNextRecurringDue_NilRule_ZeroFallback(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	result, _ := nextRecurringDue(now, "", 0)
	assert.Nil(t, result)
}

func TestNextRecurringDue_NilRule_PositiveFallback(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	result, days := nextRecurringDue(now, "", 7)
	require.NotNil(t, result)
	assert.Equal(t, 7, days)
	expected := now.AddDate(0, 0, 7)
	assert.Equal(t, expected.Year(), result.Year())
	assert.Equal(t, expected.Month(), result.Month())
	assert.Equal(t, expected.Day(), result.Day())
}

func TestNextRecurringDue_DaysRule(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	result, days := nextRecurringDue(now, "days:14", 0)
	require.NotNil(t, result)
	assert.Equal(t, 14, days)
	expected := now.AddDate(0, 0, 14)
	assert.Equal(t, expected.Day(), result.Day())
}

// ── parsePositiveInt ──────────────────────────────────────────────────────────

func TestParsePositiveInt_Valid(t *testing.T) {
	n, ok := parsePositiveInt("5")
	assert.True(t, ok)
	assert.Equal(t, 5, n)
}

func TestParsePositiveInt_Zero(t *testing.T) {
	_, ok := parsePositiveInt("0")
	assert.False(t, ok)
}

func TestParsePositiveInt_Negative(t *testing.T) {
	_, ok := parsePositiveInt("-1")
	assert.False(t, ok)
}

func TestParsePositiveInt_NonNumeric(t *testing.T) {
	_, ok := parsePositiveInt("abc")
	assert.False(t, ok)
}

// ── resolveShortcutBadge ──────────────────────────────────────────────────────

func TestResolveShortcutBadge_NoPatterns(t *testing.T) {
	badge := resolveShortcutBadge("https://jira.example.com/DCP-123", nil)
	assert.Equal(t, "", badge)
}

func TestResolveShortcutBadge_Match(t *testing.T) {
	patterns := []models.URLPattern{
		{ //nolint:exhaustruct // only fields used by resolveShortcutBadge
			URLPrefix: "https://jira.example.com/browse/",
			Shortcut:  "DCP",
		},
	}
	// shortcut + suffix after prefix: "DCP" + "123" = "DCP123"
	badge := resolveShortcutBadge(
		"https://jira.example.com/browse/123", patterns,
	)
	assert.Equal(t, "DCP123", badge)
}

func TestResolveShortcutBadge_NoMatch(t *testing.T) {
	patterns := []models.URLPattern{
		{ //nolint:exhaustruct // only fields used by resolveShortcutBadge
			URLPrefix: "https://jira.example.com/browse/",
			Shortcut:  "DCP",
		},
	}
	badge := resolveShortcutBadge("https://github.com/org/repo/pull/1", patterns)
	assert.Equal(t, "", badge)
}

// ── parseRecurOnly ────────────────────────────────────────────────────────────

func TestParseRecurOnly_Empty(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	days, rule, err := parseRecurOnly("", now)
	require.NoError(t, err)
	assert.Equal(t, 0, days)
	assert.Equal(t, "", rule)
}

func TestParseRecurOnly_Days(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	days, rule, err := parseRecurOnly("7", now)
	require.NoError(t, err)
	assert.Equal(t, 7, days)
	assert.Equal(t, "days:7", rule)
}

func TestParseRecurOnly_HumanRecurring(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	days, rule, err := parseRecurOnly("every thursday", now)
	require.NoError(t, err)
	assert.Equal(t, 7, days)
	assert.Equal(t, "weekday:4", rule)
}

func TestParseRecurOnly_NonRecurringHuman(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	_, _, err := parseRecurOnly("tomorrow", now)
	require.Error(t, err)
}

func TestParseRecurOnly_InvalidInput(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	_, _, err := parseRecurOnly("notadate", now)
	require.Error(t, err)
}

// ── ordinalToName ─────────────────────────────────────────────────────────────

func TestOrdinalToName_AllCases(t *testing.T) {
	assert.Equal(t, "first", ordinalToName(1))
	assert.Equal(t, "second", ordinalToName(ordinalSecond))
	assert.Equal(t, "third", ordinalToName(ordinalThird))
	assert.Equal(t, "fourth", ordinalToName(ordinalFourth))
	assert.Equal(t, "fifth", ordinalToName(ordinalFifth))
	assert.Equal(t, "last", ordinalToName(-1))
	assert.Equal(t, "", ordinalToName(99))
}

// ── nthWeekdayOfMonth ─────────────────────────────────────────────────────────

func TestNthWeekdayOfMonth_FirstMonday(t *testing.T) {
	// May 2026: first Monday is May 4
	d, ok := nthWeekdayOfMonth(2026, time.May, time.Monday, 1, time.UTC)
	assert.True(t, ok)
	assert.Equal(t, 4, d.Day())
}

func TestNthWeekdayOfMonth_LastFriday(t *testing.T) {
	// May 2026: last Friday is May 29
	d, ok := nthWeekdayOfMonth(2026, time.May, time.Friday, -1, time.UTC)
	assert.True(t, ok)
	assert.Equal(t, 29, d.Day())
}

func TestNthWeekdayOfMonth_InvalidOrdinal(t *testing.T) {
	_, ok := nthWeekdayOfMonth(2026, time.May, time.Monday, 0, time.UTC)
	assert.False(t, ok)
}

func TestNthWeekdayOfMonth_OrdinalTooHigh(t *testing.T) {
	_, ok := nthWeekdayOfMonth(2026, time.February, time.Monday, 5, time.UTC)
	assert.False(t, ok)
}

// ── nextRecurringDue (weekday/monthweekday rules) ─────────────────────────────

func TestNextRecurringDue_WeekdayRule(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)   // Friday
	result, days := nextRecurringDue(now, "weekday:4", 0) // Thursday
	require.NotNil(t, result)
	assert.Equal(t, daysInWeek, days)
	assert.Equal(t, time.Thursday, result.Weekday())
}

func TestNextRecurringDue_MonthWeekdayRule(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	// "monthweekday:1:0" = first Sunday
	result, days := nextRecurringDue(now, "monthweekday:1:0", 0)
	require.NotNil(t, result)
	assert.Equal(t, 0, days)
	assert.Equal(t, time.Sunday, result.Weekday())
}

func TestNextRecurringDue_InvalidWeekdayParts(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	result, _ := nextRecurringDue(now, "weekday", 3)
	assert.Nil(t, result)
}

func TestNextRecurringDue_InvalidDaysParts(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	result, _ := nextRecurringDue(now, "days", 3)
	assert.Nil(t, result)
}

func TestNextRecurringDue_UnknownRule(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	result, _ := nextRecurringDue(now, "unknown:1", 3)
	assert.Nil(t, result)
}
