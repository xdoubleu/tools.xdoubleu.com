package templates_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/templates"
)

// ── SetConfig ────────────────────────────────────────────────────────────────

func TestSetConfig_DoesNotPanic(_ *testing.T) {
	// SetConfig sets package-level globals; just verify it doesn't panic.
	templates.SetConfig("v1.2.3")
}

// ── RenderTitleLinks ─────────────────────────────────────────────────────────

func TestRenderTitleLinks_PlainText(t *testing.T) {
	out := templates.RenderTitleLinks("hello world")
	assert.Equal(t, "hello world", out)
}

func TestRenderTitleLinks_WithMarkdownLink(t *testing.T) {
	out := templates.RenderTitleLinks("[Click here](https://example.com)")
	assert.Contains(t, out, `href="https://example.com"`)
	assert.Contains(t, out, "Click here")
}

func TestRenderTitleLinks_WithRelativeURL(t *testing.T) {
	// URLs without http/https get https:// prepended.
	out := templates.RenderTitleLinks("[link](example.com/path)")
	assert.Contains(t, out, `href="https://example.com/path"`)
}

func TestRenderTitleLinks_MultipleLinks(t *testing.T) {
	out := templates.RenderTitleLinks(
		"See [A](https://a.com) and [B](https://b.com) for details",
	)
	assert.Contains(t, out, `href="https://a.com"`)
	assert.Contains(t, out, `href="https://b.com"`)
	assert.Contains(t, out, "See")
	assert.Contains(t, out, "for details")
}

func TestRenderTitleLinks_EmptyString(t *testing.T) {
	assert.Equal(t, "", templates.RenderTitleLinks(""))
}

// ── HasMdLink ────────────────────────────────────────────────────────────────

func TestHasMdLink_True(t *testing.T) {
	assert.True(t, templates.HasMdLink("[title](https://example.com)"))
}

func TestHasMdLink_False(t *testing.T) {
	assert.False(t, templates.HasMdLink("plain text"))
}

// ── ToFraction ───────────────────────────────────────────────────────────────

func TestToFraction_Zero(t *testing.T) {
	assert.Equal(t, "0", templates.ToFraction(0))
}

func TestToFraction_Negative(t *testing.T) {
	assert.Equal(t, "0", templates.ToFraction(-1))
}

func TestToFraction_Whole(t *testing.T) {
	assert.Equal(t, "2", templates.ToFraction(2.0))
}

func TestToFraction_Half(t *testing.T) {
	assert.Equal(t, "½", templates.ToFraction(0.5))
}

func TestToFraction_WholePlusHalf(t *testing.T) {
	assert.Equal(t, "1½", templates.ToFraction(1.5))
}

func TestToFraction_RoundsUp(t *testing.T) {
	// 0.9375 → nearest 1/8 = 1 → whole 1, frac 0
	assert.Equal(t, "1", templates.ToFraction(0.9375))
}

// ── RecurInputDisplay ─────────────────────────────────────────────────────────

func TestRecurInputDisplay_Empty(t *testing.T) {
	assert.Equal(t, "", templates.RecurInputDisplay(""))
}

func TestRecurInputDisplay_DaysRule(t *testing.T) {
	assert.Equal(t, "every 7 days", templates.RecurInputDisplay("days:7"))
}

func TestRecurInputDisplay_WeekdayRule(t *testing.T) {
	assert.Equal(t, "every thursday", templates.RecurInputDisplay("weekday:4"))
}

func TestRecurInputDisplay_MonthWeekdayRule(t *testing.T) {
	assert.Equal(
		t,
		"every first sunday",
		templates.RecurInputDisplay("monthweekday:1:0"),
	)
}

func TestRecurInputDisplay_MonthWeekdayLastFriday(t *testing.T) {
	assert.Equal(
		t,
		"every last friday",
		templates.RecurInputDisplay("monthweekday:-1:5"),
	)
}

func TestRecurInputDisplay_UnknownRule(t *testing.T) {
	// Unknown rule format is returned as-is.
	assert.Equal(t, "something:else", templates.RecurInputDisplay("something:else"))
}

// ── HumanDate ────────────────────────────────────────────────────────────────

func TestHumanDate_Nil(t *testing.T) {
	assert.Equal(t, "", templates.HumanDate(nil))
}

func TestHumanDate_Today(t *testing.T) {
	now := time.Now()
	assert.Equal(t, "Today", templates.HumanDate(&now))
}

func TestHumanDate_Yesterday(t *testing.T) {
	yesterday := time.Now().AddDate(0, 0, -1)
	assert.Equal(t, "Yesterday", templates.HumanDate(&yesterday))
}

func TestHumanDate_Tomorrow(t *testing.T) {
	tomorrow := time.Now().AddDate(0, 0, 1)
	assert.Equal(t, "Tomorrow", templates.HumanDate(&tomorrow))
}

func TestHumanDate_WithinWeek(t *testing.T) {
	// 3 days in the future: should return day abbreviation
	future := time.Now().AddDate(0, 0, 3)
	result := templates.HumanDate(&future)
	assert.NotEmpty(t, result)
	assert.NotEqual(t, "Today", result)
	assert.NotEqual(t, "Tomorrow", result)
}

func TestHumanDate_FarFuture(t *testing.T) {
	// >7 days in the future: returns "2 Jan" format
	far := time.Date(2030, 6, 15, 0, 0, 0, 0, time.Local)
	result := templates.HumanDate(&far)
	assert.Equal(t, "15 Jun", result)
}

// ── IsOverdue ────────────────────────────────────────────────────────────────

func TestIsOverdue_Nil(t *testing.T) {
	assert.False(t, templates.IsOverdue(nil))
}

func TestIsOverdue_Past(t *testing.T) {
	past := time.Now().AddDate(0, 0, -1)
	assert.True(t, templates.IsOverdue(&past))
}

func TestIsOverdue_Future(t *testing.T) {
	future := time.Now().AddDate(0, 0, 1)
	assert.False(t, templates.IsOverdue(&future))
}

// ── DescFirstLine ────────────────────────────────────────────────────────────

func TestDescFirstLine_SingleLine(t *testing.T) {
	assert.Equal(t, "hello", templates.DescFirstLine("hello"))
}

func TestDescFirstLine_MultiLine(t *testing.T) {
	assert.Equal(t, "first", templates.DescFirstLine("first\nsecond"))
}

func TestDescFirstLine_Empty(t *testing.T) {
	assert.Equal(t, "", templates.DescFirstLine(""))
}
