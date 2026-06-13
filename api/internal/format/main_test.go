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

// ── ToAmount ─────────────────────────────────────────────────────────────────

func TestToAmount_Zero(t *testing.T) {
	assert.Equal(t, "0", format.ToAmount(0))
}

func TestToAmount_Negative(t *testing.T) {
	assert.Equal(t, "0", format.ToAmount(-1))
}

func TestToAmount_Half(t *testing.T) {
	// No rounding: 0.5 stays 0.5 (previously rounded up to 1 for count units).
	assert.Equal(t, "0.5", format.ToAmount(0.5))
}

func TestToAmount_Whole(t *testing.T) {
	assert.Equal(t, "1", format.ToAmount(1.0))
}

func TestToAmount_WholeTwo(t *testing.T) {
	assert.Equal(t, "2", format.ToAmount(2.0))
}

func TestToAmount_WholePlusHalf(t *testing.T) {
	assert.Equal(t, "1.5", format.ToAmount(1.5))
}

func TestToAmount_OneThirdCapsAtThreeDecimals(t *testing.T) {
	assert.Equal(t, "0.333", format.ToAmount(1.0/3))
}

func TestToAmount_AbsorbsFloatNoise(t *testing.T) {
	// 0.1+0.2 == 0.30000000000000004 in float64; capping at 3 decimals -> "0.3".
	assert.Equal(t, "0.3", format.ToAmount(0.1+0.2))
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
