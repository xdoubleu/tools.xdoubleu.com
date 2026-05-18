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
	"tools.xdoubleu.com/internal/app"
)

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
	s := &TaskService{} //nolint:exhaustruct // test only uses relevant fields
	assert.Equal(t, "every 14 days", s.FormatRecurRule("", 14))
}

func TestFormatRecurRule_EmptyRuleNoFallback(t *testing.T) {
	s := &TaskService{} //nolint:exhaustruct // test only uses relevant fields
	assert.Equal(t, "", s.FormatRecurRule("", 0))
}

func TestFormatRecurRule_InvalidRuleWithFallback(t *testing.T) {
	s := &TaskService{} //nolint:exhaustruct // test only uses relevant fields
	assert.Equal(t, "every 3 days", s.FormatRecurRule("unknown:foo", 3))
}

// ── findSection ───────────────────────────────────────────────────────────────

func TestFindSection_Found(t *testing.T) {
	sections := []models.Section{
		{Name: "Backlog"}, //nolint:exhaustruct // test only uses relevant fields
		{Name: "Done"},    //nolint:exhaustruct // test only uses relevant fields
	}
	s := findSection(sections, "backlog")
	require.NotNil(t, s)
	assert.Equal(t, "Backlog", s.Name)
}

func TestFindSection_NotFound(t *testing.T) {
	sections := []models.Section{
		{Name: "Backlog"}, //nolint:exhaustruct // test only uses relevant fields
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
	var httpErr *app.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusBadRequest, httpErr.Status)
}
