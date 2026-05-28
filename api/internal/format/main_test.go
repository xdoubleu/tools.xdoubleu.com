package format_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/format"
)

// ── SetConfig ────────────────────────────────────────────────────────────────

func TestSetConfig_DoesNotPanic(_ *testing.T) {
	// SetConfig sets package-level globals; just verify it doesn't panic.
	format.SetConfig("v1.2.3")
}

// ── RenderTitleLinks ─────────────────────────────────────────────────────────

func TestRenderTitleLinks_PlainText(t *testing.T) {
	out := format.RenderTitleLinks("hello world")
	assert.Equal(t, "hello world", out)
}

func TestRenderTitleLinks_WithMarkdownLink(t *testing.T) {
	out := format.RenderTitleLinks("[Click here](https://example.com)")
	assert.Contains(t, out, `href="https://example.com"`)
	assert.Contains(t, out, "Click here")
}

func TestRenderTitleLinks_WithRelativeURL(t *testing.T) {
	// URLs without http/https get https:// prepended.
	out := format.RenderTitleLinks("[link](example.com/path)")
	assert.Contains(t, out, `href="https://example.com/path"`)
}

func TestRenderTitleLinks_MultipleLinks(t *testing.T) {
	out := format.RenderTitleLinks(
		"See [A](https://a.com) and [B](https://b.com) for details",
	)
	assert.Contains(t, out, `href="https://a.com"`)
	assert.Contains(t, out, `href="https://b.com"`)
	assert.Contains(t, out, "See")
	assert.Contains(t, out, "for details")
}

func TestRenderTitleLinks_EmptyString(t *testing.T) {
	assert.Equal(t, "", format.RenderTitleLinks(""))
}

// ── HasMdLink ────────────────────────────────────────────────────────────────

func TestHasMdLink_True(t *testing.T) {
	assert.True(t, format.HasMdLink("[title](https://example.com)"))
}

func TestHasMdLink_False(t *testing.T) {
	assert.False(t, format.HasMdLink("plain text"))
}

// ── ToFraction ───────────────────────────────────────────────────────────────

func TestToFraction_Zero(t *testing.T) {
	assert.Equal(t, "0", format.ToFraction(0))
}

func TestToFraction_Negative(t *testing.T) {
	assert.Equal(t, "0", format.ToFraction(-1))
}

func TestToFraction_Whole(t *testing.T) {
	assert.Equal(t, "2", format.ToFraction(2.0))
}

func TestToFraction_Half(t *testing.T) {
	assert.Equal(t, "½", format.ToFraction(0.5))
}

func TestToFraction_WholePlusHalf(t *testing.T) {
	assert.Equal(t, "1½", format.ToFraction(1.5))
}

func TestToFraction_RoundsUp(t *testing.T) {
	// 0.9375 → nearest 1/8 = 1 → whole 1, frac 0
	assert.Equal(t, "1", format.ToFraction(0.9375))
}

// ── RecurInputDisplay ─────────────────────────────────────────────────────────

func TestRecurInputDisplay_Empty(t *testing.T) {
	assert.Equal(t, "", format.RecurInputDisplay(""))
}

func TestRecurInputDisplay_DaysRule(t *testing.T) {
	assert.Equal(t, "every 7 days", format.RecurInputDisplay("days:7"))
}

func TestRecurInputDisplay_WeekdayRule(t *testing.T) {
	assert.Equal(t, "every thursday", format.RecurInputDisplay("weekday:4"))
}

func TestRecurInputDisplay_MonthWeekdayRule(t *testing.T) {
	assert.Equal(
		t,
		"every first sunday",
		format.RecurInputDisplay("monthweekday:1:0"),
	)
}

func TestRecurInputDisplay_MonthWeekdayLastFriday(t *testing.T) {
	assert.Equal(
		t,
		"every last friday",
		format.RecurInputDisplay("monthweekday:-1:5"),
	)
}

func TestRecurInputDisplay_UnknownRule(t *testing.T) {
	// Unknown rule format is returned as-is.
	assert.Equal(t, "something:else", format.RecurInputDisplay("something:else"))
}

// ── HumanDate ────────────────────────────────────────────────────────────────

func TestHumanDate_Nil(t *testing.T) {
	assert.Equal(t, "", format.HumanDate(nil))
}

func TestHumanDate_Today(t *testing.T) {
	now := time.Now()
	assert.Equal(t, "Today", format.HumanDate(&now))
}

func TestHumanDate_Yesterday(t *testing.T) {
	yesterday := time.Now().AddDate(0, 0, -1)
	assert.Equal(t, "Yesterday", format.HumanDate(&yesterday))
}

func TestHumanDate_Tomorrow(t *testing.T) {
	tomorrow := time.Now().AddDate(0, 0, 1)
	assert.Equal(t, "Tomorrow", format.HumanDate(&tomorrow))
}

func TestHumanDate_WithinWeek(t *testing.T) {
	// 3 days in the future: should return day abbreviation
	future := time.Now().AddDate(0, 0, 3)
	result := format.HumanDate(&future)
	assert.NotEmpty(t, result)
	assert.NotEqual(t, "Today", result)
	assert.NotEqual(t, "Tomorrow", result)
}

func TestHumanDate_FarFuture(t *testing.T) {
	// >7 days in the future: returns "2 Jan" format
	far := time.Date(2030, 6, 15, 0, 0, 0, 0, time.Local)
	result := format.HumanDate(&far)
	assert.Equal(t, "15 Jun", result)
}

// ── IsOverdue ────────────────────────────────────────────────────────────────

func TestIsOverdue_Nil(t *testing.T) {
	assert.False(t, format.IsOverdue(nil))
}

func TestIsOverdue_Past(t *testing.T) {
	past := time.Now().AddDate(0, 0, -1)
	assert.True(t, format.IsOverdue(&past))
}

func TestIsOverdue_Future(t *testing.T) {
	future := time.Now().AddDate(0, 0, 1)
	assert.False(t, format.IsOverdue(&future))
}

// ── DescFirstLine ────────────────────────────────────────────────────────────

func TestDescFirstLine_SingleLine(t *testing.T) {
	assert.Equal(t, "hello", format.DescFirstLine("hello"))
}

func TestDescFirstLine_MultiLine(t *testing.T) {
	assert.Equal(t, "first", format.DescFirstLine("first\nsecond"))
}

func TestDescFirstLine_Empty(t *testing.T) {
	assert.Equal(t, "", format.DescFirstLine(""))
}

func TestToFraction_OneThird(t *testing.T) {
	assert.Equal(t, "⅓", format.ToFraction(1.0/3))
}

func TestToFraction_TwoThirds(t *testing.T) {
	assert.Equal(t, "⅔", format.ToFraction(2.0/3))
}

func TestToFraction_OneAndOneThird(t *testing.T) {
	assert.Equal(t, "1⅓", format.ToFraction(1.0+1.0/3))
}

// ── ToFractionCeiling ─────────────────────────────────────────────────────────

func TestToFractionCeiling_Zero(t *testing.T) {
	assert.Equal(t, "0", format.ToFractionCeiling(0))
}

func TestToFractionCeiling_Negative(t *testing.T) {
	assert.Equal(t, "0", format.ToFractionCeiling(-1))
}

func TestToFractionCeiling_Exact(t *testing.T) {
	assert.Equal(t, "½", format.ToFractionCeiling(0.5))
}

func TestToFractionCeiling_RoundsUp(t *testing.T) {
	// 0.1 → rounds up to nearest common fraction ⅛ → ⅛
	assert.Equal(t, "⅛", format.ToFractionCeiling(0.1))
}

func TestToFractionCeiling_Whole(t *testing.T) {
	assert.Equal(t, "3", format.ToFractionCeiling(3.0))
}

func TestToFractionCeiling_WholePlusFraction(t *testing.T) {
	// 1.1 → fractional part 0.1, ceiling to nearest common fraction ⅛ (0.125)
	assert.Equal(t, "1⅛", format.ToFractionCeiling(1.1))
}

func TestToFractionCeiling_AlmostWhole(t *testing.T) {
	// 0.95 → rounds up to 1
	assert.Equal(t, "1", format.ToFractionCeiling(0.95))
}

// ── RenderError ───────────────────────────────────────────────────────────────

func TestRenderError_WritesStatusAndBody(t *testing.T) {
	rec := httptest.NewRecorder()
	format.RenderError(rec, http.StatusBadRequest, "bad input")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "bad input")
}

func TestRenderError_InternalServerError(t *testing.T) {
	rec := httptest.NewRecorder()
	format.RenderError(rec, http.StatusInternalServerError, "oops")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
