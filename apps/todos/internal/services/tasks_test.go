//nolint:testpackage // tests unexported helpers
package services

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tools.xdoubleu.com/apps/todos/internal/dtos"
	"tools.xdoubleu.com/apps/todos/internal/models"
)

func TestURLToTitle_LastSegment(t *testing.T) {
	assert.Equal(
		t,
		"https://jira.company.com/browse/CR-1234",
		urlToTitle("https://jira.company.com/browse/CR-1234"),
	)
	assert.Equal(
		t,
		"https://github.com/org/repo/pull/42",
		urlToTitle("https://github.com/org/repo/pull/42"),
	)
	assert.Equal(t, "https://example.com", urlToTitle("https://example.com"))
}

func TestURLToTitle_TrailingSlash(t *testing.T) {
	assert.Equal(
		t,
		"https://jira.company.com/browse/CR-1234/",
		urlToTitle("https://jira.company.com/browse/CR-1234/"),
	)
}

func TestURLToTitle_InvalidURL(t *testing.T) {
	assert.Equal(t, "not a url", urlToTitle("not a url"))
}

func TestParseDatePtr_ValidDate(t *testing.T) {
	p := parseDatePtr("2026-05-01")
	assert.NotNil(t, p)
	assert.Equal(t, 2026, p.Year())
	assert.Equal(t, 5, int(p.Month()))
	assert.Equal(t, 1, p.Day())
}

func TestParseDatePtr_Empty(t *testing.T) {
	assert.Nil(t, parseDatePtr(""))
}

func TestParseDatePtr_Invalid(t *testing.T) {
	assert.Nil(t, parseDatePtr("not-a-date"))
}

func TestParseHumanDate_EveryThursday(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC) // Friday
	d, recurring, err := parseHumanDate("every thursday", now, true)
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, 7, recurring.recurDays)
	assert.Equal(t, "weekday:4", recurring.recurRule)
	assert.Equal(t, time.Thursday, d.Weekday())
}

func TestParseHumanDate_EveryFirstSunday(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	d, recurring, err := parseHumanDate("every first sunday", now, true)
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, "monthweekday:1:0", recurring.recurRule)
	assert.Equal(t, 7, d.Day())
	assert.Equal(t, time.Sunday, d.Weekday())
}

func TestParseHumanDate_DeadlineRejectsRecurring(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	_, _, err := parseHumanDate("every thursday", now, false)
	require.Error(t, err)
}

func TestParseScheduleDTO_DueEveryThursdaySetsRecurDays(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	//nolint:exhaustruct // only schedule fields needed
	dto := dtos.SaveTaskDto{DueDate: "every thursday"}
	due, _, recurDays, recurRule, err := parseScheduleDTO(dto, now)
	require.NoError(t, err)
	require.NotNil(t, due)
	assert.Equal(t, 7, recurDays)
	assert.Equal(t, "weekday:4", recurRule)
	assert.Equal(t, time.Thursday, due.Weekday())
}

func TestParseScheduleDTO_DeadlineHumanDate(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	//nolint:exhaustruct // only schedule fields needed
	dto := dtos.SaveTaskDto{Deadline: "tomorrow"}
	_, deadline, recurDays, recurRule, err := parseScheduleDTO(dto, now)
	require.NoError(t, err)
	require.NotNil(t, deadline)
	assert.Equal(t, 0, recurDays)
	assert.Equal(t, "", recurRule)
	assert.Equal(t, 9, deadline.Day())
}

func TestParseQuickInput_DeadlineShortcutToday(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	title, dto := parseQuickInput("Ship patch !today", nil, now)
	assert.Equal(t, "Ship patch", title)
	assert.Equal(t, "2026-05-08", dto.Deadline)
}

func TestParseQuickInput_DeadlineShortcutNextWeekday(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC) // Friday
	title, dto := parseQuickInput("Ship patch !next thursday", nil, now)
	assert.Equal(t, "Ship patch", title)
	assert.Equal(t, "2026-05-14", dto.Deadline)
}

func TestParseQuickInput_RecurringEveryThursday(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	title, dto := parseQuickInput("Workout every thursday", nil, now)
	assert.Equal(t, "Workout", title)
	assert.Equal(t, "2026-05-14", dto.DueDate)
	assert.Equal(t, "every thursday", dto.Recur)
}

func TestParseQuickInput_RecurringEveryFirstSunday(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	title, dto := parseQuickInput("Call grandma every first sunday", nil, now)
	assert.Equal(t, "Call grandma", title)
	assert.Equal(t, "2026-06-07", dto.DueDate)
	assert.Equal(t, "every first sunday", dto.Recur)
}

// ── parseFancyURL ─────────────────────────────────────────────────────────────

func TestParseFancyURL_Valid(t *testing.T) {
	title, rawURL, rest, ok := parseFancyURL(
		"[Fix homepage bug](https://jira.example.com/PROJ-42)",
	)
	require.True(t, ok)
	assert.Equal(t, "Fix homepage bug", title)
	assert.Equal(t, "https://jira.example.com/PROJ-42", rawURL)
	assert.Equal(t, "", rest)
}

func TestParseFancyURL_WithTrailingShortcuts(t *testing.T) {
	title, rawURL, rest, ok := parseFancyURL(
		"[Fix bug](https://jira.example.com/PROJ-42) p1 @cr",
	)
	require.True(t, ok)
	assert.Equal(t, "Fix bug", title)
	assert.Equal(t, "https://jira.example.com/PROJ-42", rawURL)
	assert.Equal(t, "p1 @cr", rest)
}

func TestParseFancyURL_PlainURL(t *testing.T) {
	_, _, _, ok := parseFancyURL("https://example.com/path")
	assert.False(t, ok)
}

func TestParseFancyURL_PlainTitle(t *testing.T) {
	_, _, _, ok := parseFancyURL("buy milk today")
	assert.False(t, ok)
}

func TestParseFancyURL_NestedBrackets(t *testing.T) {
	const input = "[[TAG1] [Category] Some task title [REF#123] | Project ABC123]" +
		"(https://example.com/task/123)"
	title, rawURL, rest, ok := parseFancyURL(input)
	require.True(t, ok)
	assert.Equal(
		t,
		"[TAG1] [Category] Some task title [REF#123] | Project ABC123",
		title,
	)
	assert.Equal(t, "https://example.com/task/123", rawURL)
	assert.Equal(t, "", rest)
}

func TestParseFancyURL_MissingURL(t *testing.T) {
	title, rawURL, rest, ok := parseFancyURL("[Title](not-a-url)")
	require.True(t, ok)
	assert.Equal(t, "Title", title)
	assert.Equal(t, "https://not-a-url", rawURL)
	assert.Equal(t, "", rest)
}

// ── shortcutQueryPattern ──────────────────────────────────────────────────────

func TestShortcutQueryPattern_Matches(t *testing.T) {
	m := shortcutQueryPattern.FindStringSubmatch("DCP1234")
	require.NotNil(t, m)
	assert.Equal(t, "DCP", m[1])
	assert.Equal(t, "1234", m[2])
}

func TestShortcutQueryPattern_WithDash(t *testing.T) {
	m := shortcutQueryPattern.FindStringSubmatch("PROJ-42")
	require.NotNil(t, m)
	assert.Equal(t, "PROJ", m[1])
	assert.Equal(t, "-42", m[2])
}

func TestShortcutQueryPattern_NoMatch_LowerCase(t *testing.T) {
	assert.Nil(t, shortcutQueryPattern.FindStringSubmatch("dcp1234"))
}

func TestShortcutQueryPattern_NoMatch_PlainText(t *testing.T) {
	assert.Nil(t, shortcutQueryPattern.FindStringSubmatch("fix bug"))
}

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

// ── recurRuleToInput ──────────────────────────────────────────────────────────

func TestRecurRuleToInput_Days(t *testing.T) {
	assert.Equal(t, "every 7 days", recurRuleToInput("days:7"))
}

func TestRecurRuleToInput_Weekday(t *testing.T) {
	assert.Equal(t, "every thursday", recurRuleToInput("weekday:4"))
}

func TestRecurRuleToInput_MonthWeekday(t *testing.T) {
	assert.Equal(t, "every first sunday", recurRuleToInput("monthweekday:1:0"))
}

func TestRecurRuleToInput_MonthWeekdayLast(t *testing.T) {
	assert.Equal(t, "every last friday", recurRuleToInput("monthweekday:-1:5"))
}

func TestRecurRuleToInput_Unknown(t *testing.T) {
	assert.Equal(t, "", recurRuleToInput("unknown:1"))
}

func TestRecurRuleToInput_InvalidParts(t *testing.T) {
	assert.Equal(t, "", recurRuleToInput("weekday"))
}

// ── FormatRecurRule ───────────────────────────────────────────────────────────

func TestFormatRecurRule_WithRule(t *testing.T) {
	s := &TaskService{} //nolint:exhaustruct // only testing FormatRecurRule
	assert.Equal(t, "every 7 days", s.FormatRecurRule("days:7", 0))
}

func TestFormatRecurRule_EmptyRuleWithFallback(t *testing.T) {
	s := &TaskService{} //nolint:exhaustruct
	assert.Equal(t, "every 14 days", s.FormatRecurRule("", 14))
}

func TestFormatRecurRule_EmptyRuleNoFallback(t *testing.T) {
	s := &TaskService{} //nolint:exhaustruct
	assert.Equal(t, "", s.FormatRecurRule("", 0))
}

func TestFormatRecurRule_InvalidRuleWithFallback(t *testing.T) {
	s := &TaskService{} //nolint:exhaustruct
	assert.Equal(t, "every 3 days", s.FormatRecurRule("unknown:foo", 3))
}

// ── findSection ───────────────────────────────────────────────────────────────

func TestFindSection_Found(t *testing.T) {
	sections := []models.Section{
		{Name: "Backlog"}, //nolint:exhaustruct
		{Name: "Done"},    //nolint:exhaustruct
	}
	s := findSection(sections, "backlog")
	require.NotNil(t, s)
	assert.Equal(t, "Backlog", s.Name)
}

func TestFindSection_NotFound(t *testing.T) {
	sections := []models.Section{
		{Name: "Backlog"}, //nolint:exhaustruct
	}
	assert.Nil(t, findSection(sections, "Archive"))
}

func TestFindSection_Empty(t *testing.T) {
	assert.Nil(t, findSection(nil, "anything"))
}

// ── parseDateFromTitle ────────────────────────────────────────────────────────

func TestParseDateFromTitle_Tomorrow(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	title, due, rule, days := parseDateFromTitle("Buy milk tomorrow", now)
	assert.Equal(t, "Buy milk", title)
	require.NotNil(t, due)
	assert.Equal(t, 9, due.Day())
	assert.Equal(t, "", rule)
	assert.Equal(t, 0, days)
}

func TestParseDateFromTitle_Today(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	title, due, rule, days := parseDateFromTitle("Stand-up today", now)
	assert.Equal(t, "Stand-up", title)
	require.NotNil(t, due)
	assert.Equal(t, 8, due.Day())
	assert.Equal(t, "", rule)
	assert.Equal(t, 0, days)
}

func TestParseDateFromTitle_NextWeekday(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC) // Friday
	title, due, rule, days := parseDateFromTitle("Meeting next monday", now)
	assert.Equal(t, "Meeting", title)
	require.NotNil(t, due)
	assert.Equal(t, time.Monday, due.Weekday())
	assert.Equal(t, "", rule)
	assert.Equal(t, 0, days)
}

func TestParseDateFromTitle_NoDate(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	title, due, rule, days := parseDateFromTitle("Write tests", now)
	assert.Equal(t, "Write tests", title)
	assert.Nil(t, due)
	assert.Equal(t, "", rule)
	assert.Equal(t, 0, days)
}

func TestParseEveryDate_UnknownBody(t *testing.T) {
	// "every bazinga" matches the everyPattern regex but "bazinga" is not a
	// weekday name and not "N days" — so parseEveryDate hits the final ErrSyntax.
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	d, recurring, err := parseHumanDate("every bazinga", now, true)
	require.Error(t, err)
	assert.Nil(t, d)
	assert.Empty(t, recurring.recurRule)
}

func TestParseEveryDate_DisallowedWhenNotRecurring(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	// allowRecurring=false must return an error even for a valid "every X" phrase.
	_, _, err := parseHumanDate("every thursday", now, false)
	require.Error(t, err)
}

func TestParseDateFromTitle_EveryThursday(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	title, due, rule, days := parseDateFromTitle("Workout every thursday", now)
	assert.Equal(t, "Workout", title)
	require.NotNil(t, due)
	assert.Equal(t, "weekday:4", rule)
	assert.Equal(t, 7, days)
}

// ── parseDeadlineTok ──────────────────────────────────────────────────────────

func TestParseDeadlineTok_ValidHuman(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	toks := []string{"!today"}
	d, skip, ok := parseDeadlineTok("!today", toks, 0, now)
	assert.True(t, ok)
	assert.Equal(t, "2026-05-08", d)
	assert.Equal(t, 0, skip)
}

func TestParseDeadlineTok_ValidISODate(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	toks := []string{"!2026-12-31"}
	d, skip, ok := parseDeadlineTok("!2026-12-31", toks, 0, now)
	assert.True(t, ok)
	assert.Equal(t, "2026-12-31", d)
	assert.Equal(t, 0, skip)
}

func TestParseDeadlineTok_NextWeekday(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	toks := []string{"!next", "monday"}
	d, skip, ok := parseDeadlineTok("!next", toks, 0, now)
	assert.True(t, ok)
	assert.Equal(t, 1, skip)
	assert.NotEmpty(t, d)
}

func TestParseDeadlineTok_GarbageToken(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	toks := []string{"!xyz-garbage-date"}
	d, skip, ok := parseDeadlineTok("!xyz-garbage-date", toks, 0, now)
	assert.False(t, ok)
	assert.Equal(t, "", d)
	assert.Equal(t, 0, skip)
}

// ── nextMonthlyWeekday ────────────────────────────────────────────────────────

func TestNextMonthlyWeekday_CurrentMonthFuture(t *testing.T) {
	// May 8 2026 is a Friday. First Sunday in May = May 3, already past.
	// So nextMonthlyWeekday should return the first Sunday of June = June 7.
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	result, ok := nextMonthlyWeekday(now, time.Sunday, 1)
	assert.True(t, ok)
	assert.Equal(t, time.Sunday, result.Weekday())
}

func TestNextMonthlyWeekday_InvalidOrdinal(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	_, ok := nextMonthlyWeekday(now, time.Monday, 0)
	assert.False(t, ok)
}

// ── UpdateSubtask validation (service-level) ──────────────────────────────────

func TestUpdateSubtask_EmptyTitle(t *testing.T) {
	s := &TaskService{} //nolint:exhaustruct // only testing validation
	_, err := s.UpdateSubtask(
		context.Background(),
		uuid.New(),
		uuid.New(),
		"user-1",
		nil,
		dtos.UpdateSubtaskDto{Title: "   ", Label: "", Priority: 0,
			DueDate: "", Deadline: "", Description: ""},
	)
	require.Error(t, err)
	var httpErr *HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusBadRequest, httpErr.Status)
}
