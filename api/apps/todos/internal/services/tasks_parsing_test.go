//nolint:testpackage // tests unexported helpers
package services

import (
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

// ── parseLabelsInput ──────────────────────────────────────────────────────────

func TestParseLabelsInput_Empty(t *testing.T) {
	assert.Equal(t, []string{}, parseLabelsInput(""))
}

func TestParseLabelsInput_Single(t *testing.T) {
	assert.Equal(t, []string{"bug"}, parseLabelsInput("bug"))
}

func TestParseLabelsInput_Multiple(t *testing.T) {
	assert.Equal(t, []string{"bug", "feature"}, parseLabelsInput("bug,feature"))
}

func TestParseLabelsInput_TrimsSpaces(t *testing.T) {
	assert.Equal(t, []string{"bug", "feature"}, parseLabelsInput(" bug , feature "))
}

func TestParseLabelsInput_SkipsBlank(t *testing.T) {
	assert.Equal(t, []string{"bug"}, parseLabelsInput("bug,,"))
}

// ── parseSectionID ────────────────────────────────────────────────────────────

func TestParseSectionID_Empty(t *testing.T) {
	assert.Nil(t, parseSectionID(""))
}

func TestParseSectionID_Invalid(t *testing.T) {
	assert.Nil(t, parseSectionID("not-a-uuid"))
}

func TestParseSectionID_Valid(t *testing.T) {
	id := uuid.New()
	result := parseSectionID(id.String())
	require.NotNil(t, result)
	assert.Equal(t, id, *result)
}

// ── dtoToLinks ────────────────────────────────────────────────────────────────

func TestDtoToLinks_Empty(t *testing.T) {
	//nolint:exhaustruct // only relevant fields set in test
	dto := dtos.SaveTaskDto{}
	assert.Empty(t, dtoToLinks(dto, uuid.New()))
}

func TestDtoToLinks_SkipsEmptyURL(t *testing.T) {
	//nolint:exhaustruct // only relevant fields set in test
	dto := dtos.SaveTaskDto{LinkURLs: []string{"", ""}}
	assert.Empty(t, dtoToLinks(dto, uuid.New()))
}

func TestDtoToLinks_SingleLink(t *testing.T) {
	taskID := uuid.New()
	//nolint:exhaustruct // only relevant fields set in test
	dto := dtos.SaveTaskDto{
		LinkURLs:   []string{"https://example.com"},
		LinkLabels: []string{"Example"},
	}
	links := dtoToLinks(dto, taskID)
	require.Len(t, links, 1)
	assert.Equal(t, "https://example.com", links[0].URL)
	assert.Equal(t, "Example", links[0].Label)
	assert.Equal(t, taskID, links[0].TaskID)
	assert.Equal(t, 0, links[0].SortOrder)
}

func TestDtoToLinks_MultipleLinks(t *testing.T) {
	taskID := uuid.New()
	//nolint:exhaustruct // only relevant fields set in test
	dto := dtos.SaveTaskDto{
		LinkURLs:   []string{"https://a.com", "https://b.com"},
		LinkLabels: []string{"A", "B"},
	}
	links := dtoToLinks(dto, taskID)
	require.Len(t, links, 2)
	assert.Equal(t, 0, links[0].SortOrder)
	assert.Equal(t, 1, links[1].SortOrder)
}

func TestDtoToLinks_NoLabelForIndex(t *testing.T) {
	taskID := uuid.New()
	//nolint:exhaustruct // only relevant fields set in test
	dto := dtos.SaveTaskDto{
		LinkURLs:   []string{"https://a.com", "https://b.com"},
		LinkLabels: []string{"A"},
	}
	links := dtoToLinks(dto, taskID)
	require.Len(t, links, 2)
	assert.Equal(t, "A", links[0].Label)
	assert.Equal(t, "", links[1].Label)
}

// ── parseHumanDate edge cases ─────────────────────────────────────────────────

func TestParseHumanDate_ISODate(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	d, recurring, err := parseHumanDate("2026-06-15", now, false)
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, 2026, d.Year())
	assert.Equal(t, time.June, d.Month())
	assert.Equal(t, 15, d.Day())
	assert.Equal(t, "", recurring.recurRule)
}

func TestParseHumanDate_Today(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	d, _, err := parseHumanDate("today", now, false)
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, 8, d.Day())
}

func TestParseHumanDate_Tomorrow(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	d, _, err := parseHumanDate("tomorrow", now, false)
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, 9, d.Day())
}

func TestParseHumanDate_WeekdayName(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC) // Friday
	d, _, err := parseHumanDate("monday", now, false)
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, time.Monday, d.Weekday())
}

func TestParseHumanDate_Invalid(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	_, _, err := parseHumanDate("not-a-date", now, false)
	require.Error(t, err)
}

func TestParseHumanDate_Empty(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	d, _, err := parseHumanDate("", now, false)
	require.NoError(t, err)
	assert.Nil(t, d)
}

func TestParseHumanDate_EveryNDays(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	d, recurring, err := parseHumanDate("every 7 days", now, true)
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, 7, recurring.recurDays)
	assert.Equal(t, "days:7", recurring.recurRule)
}

func TestParseHumanDate_NextThursday(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC) // Friday
	d, _, err := parseHumanDate("next thursday", now, false)
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, time.Thursday, d.Weekday())
}

// ── parseQuickInput additional cases ─────────────────────────────────────────

func TestParseQuickInput_Priority(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	title, dto := parseQuickInput("Fix critical bug p1", nil, now)
	assert.Equal(t, "Fix critical bug", title)
	assert.Equal(t, models.PriorityP1, dto.Priority)
}

func TestParseQuickInput_Label(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	title, dto := parseQuickInput("Fix bug @backend", nil, now)
	assert.Equal(t, "Fix bug", title)
	assert.Equal(t, "backend", dto.Label)
}

func TestParseQuickInput_ISODueDate(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	title, dto := parseQuickInput("Do task 2026-06-01", nil, now)
	assert.Equal(t, "Do task", title)
	assert.Equal(t, "2026-06-01", dto.DueDate)
}

func TestParseQuickInput_TildePrefixKeptInTitle(t *testing.T) {
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	title, _ := parseQuickInput("~tag something", nil, now)
	assert.Contains(t, title, "~tag")
}
