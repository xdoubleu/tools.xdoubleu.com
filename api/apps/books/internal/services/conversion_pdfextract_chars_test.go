//nolint:testpackage // testing unexported service helpers
package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGroupLines_CommaStaysOnLine: a comma's short, low box must not split into
// its own line. Geometry is from a production PDF.
func TestGroupLines_CommaStaysOnLine(t *testing.T) {
	chars := []pdfChar{
		{text: "h", left: 0, top: 584.07, right: 8, bottom: 574.36, font: ""},
		{text: "i", left: 8, top: 585.77, right: 12, bottom: 574.50, font: ""},
		{text: ",", left: 12, top: 576.36, right: 15, bottom: 571.92, font: ""},
		{text: "b", left: 18, top: 586.01, right: 26, bottom: 574.12, font: ""},
		{text: "y", left: 26, top: 580.82, right: 34, bottom: 570.26, font: ""},
		{text: "e", left: 34, top: 582.03, right: 41, bottom: 574.32, font: ""},
	}

	lines := groupLines(chars)
	if len(lines) != 1 {
		t.Fatalf(
			"expected the comma to stay on the same line, got %d lines: %+v",
			len(lines),
			lines,
		)
	}
	if got, want := lines[0].text, "hi, bye"; got != want {
		t.Fatalf("line text = %q, want %q", got, want)
	}
}

// TestGroupLines_ApostropheStaysOnLine: a high-set apostrophe must not split
// into its own line. Geometry is from a production PDF.
func TestGroupLines_ApostropheStaysOnLine(t *testing.T) {
	chars := []pdfChar{
		{text: "k", left: 0, top: 586.01, right: 8, bottom: 574.12, font: ""},
		{text: "i", left: 8, top: 585.77, right: 12, bottom: 574.50, font: ""},
		{text: "d", left: 12, top: 586.01, right: 20, bottom: 574.12, font: ""},
		{text: "s", left: 20, top: 582.03, right: 27, bottom: 574.32, font: ""},
		{text: "’", left: 27, top: 586.01, right: 30, bottom: 580.20, font: ""},
		{text: "t", left: 33, top: 584.07, right: 38, bottom: 574.36, font: ""},
		{text: "o", left: 38, top: 582.03, right: 46, bottom: 574.32, font: ""},
		{text: "y", left: 46, top: 580.82, right: 54, bottom: 570.26, font: ""},
		{text: "s", left: 54, top: 582.03, right: 61, bottom: 574.32, font: ""},
	}

	lines := groupLines(chars)
	if len(lines) != 1 {
		t.Fatalf(
			"expected the apostrophe to stay on the same line, got %d lines: %+v",
			len(lines),
			lines,
		)
	}
	if got, want := lines[0].text, "kids’ toys"; got != want {
		t.Fatalf("line text = %q, want %q", got, want)
	}
}

// TestGroupLines_AllZeroHeightChars_FallsBackToNormalClustering: with all
// zero-height chars every char is "small", so groupLines clusters them all.
func TestGroupLines_AllZeroHeightChars_FallsBackToNormalClustering(t *testing.T) {
	chars := []pdfChar{
		{text: "a", left: 0, top: 580, right: 5, bottom: 580, font: ""},
		{text: "b", left: 5, top: 580, right: 10, bottom: 580, font: ""},
	}

	lines := groupLines(chars)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %+v", len(lines), lines)
	}
	if got, want := lines[0].text, "ab"; got != want {
		t.Fatalf("line text = %q, want %q", got, want)
	}
}

// TestBuildLine_FontBoundaryInsertsSpace: "of" and "Growth" in different runs
// with no gap must not fuse into "ofGrowth".
func TestBuildLine_FontBoundaryInsertsSpace(t *testing.T) {
	chars := []pdfChar{
		{
			text:   "o",
			left:   0,
			top:    584.07,
			right:  8,
			bottom: 574.36,
			font:   "Times-Roman",
		},
		{
			text:   "f",
			left:   8,
			top:    584.07,
			right:  12,
			bottom: 574.36,
			font:   "Times-Roman",
		},
		{
			text:   "G",
			left:   12,
			top:    584.07,
			right:  20,
			bottom: 574.36,
			font:   "Times-Italic",
		},
		{
			text:   "r",
			left:   20,
			top:    584.07,
			right:  26,
			bottom: 574.36,
			font:   "Times-Italic",
		},
	}

	line := buildLine(chars)
	if got, want := line.text, "of Gr"; got != want {
		t.Fatalf("line text = %q, want %q", got, want)
	}
}

// TestBuildLine_NoFontInfo_FallsBackToGapCheck: without font info, spacing is
// gap-only.
func TestBuildLine_NoFontInfo_FallsBackToGapCheck(t *testing.T) {
	chars := []pdfChar{
		{text: "o", left: 0, top: 584.07, right: 8, bottom: 574.36, font: ""},
		{text: "f", left: 8, top: 584.07, right: 12, bottom: 574.36, font: ""},
		{text: "G", left: 12, top: 584.07, right: 20, bottom: 574.36, font: ""},
		{text: "r", left: 20, top: 584.07, right: 26, bottom: 574.36, font: ""},
	}

	line := buildLine(chars)
	if got, want := line.text, "ofGr"; got != want {
		t.Fatalf("line text = %q, want %q", got, want)
	}
}

// TestGroupLines_SmallCharFarFromAnyLine_StartsOwnLine: a distant small char
// starts its own line.
func TestGroupLines_SmallCharFarFromAnyLine_StartsOwnLine(t *testing.T) {
	chars := []pdfChar{
		{text: "h", left: 0, top: 584.07, right: 8, bottom: 574.36, font: ""},
		{text: "i", left: 8, top: 585.77, right: 12, bottom: 574.50, font: ""},
		{text: ",", left: 0, top: 400, right: 3, bottom: 396, font: ""},
	}

	lines := groupLines(chars)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %+v", len(lines), lines)
	}
}

// TestGroupLines_QuotationMarksStayOnLine: high-set double quotes attach to
// their line.
func TestGroupLines_QuotationMarksStayOnLine(t *testing.T) {
	chars := []pdfChar{
		{text: "\"", left: 0, top: 586.01, right: 3, bottom: 580.20, font: ""},
		{text: "h", left: 3, top: 584.07, right: 11, bottom: 574.36, font: ""},
		{text: "i", left: 11, top: 585.77, right: 15, bottom: 574.50, font: ""},
		{text: "b", left: 18, top: 586.01, right: 26, bottom: 574.12, font: ""},
		{text: "y", left: 26, top: 580.82, right: 34, bottom: 570.26, font: ""},
		{text: "e", left: 34, top: 582.03, right: 41, bottom: 574.32, font: ""},
		{text: "\"", left: 41, top: 586.01, right: 44, bottom: 580.20, font: ""},
	}

	lines := groupLines(chars)
	if len(lines) != 1 {
		t.Fatalf(
			"expected both quotation marks to stay on the same line, got %d lines: %+v",
			len(lines),
			lines,
		)
	}
	if got, want := lines[0].text, "\"hi bye\""; got != want {
		t.Fatalf("line text = %q, want %q", got, want)
	}
}

// TestGroupLines_LargeTitleAboveSmallBodyText_StaysOnOneLine: a large title
// above small body text must not fracture ("Wh / y"), since the page median
// height comes from the body font.
func TestGroupLines_LargeTitleAboveSmallBodyText_StaysOnOneLine(t *testing.T) {
	chars := []pdfChar{
		{text: "W", left: 0, top: 520, right: 14, bottom: 500, font: ""},
		{text: "h", left: 14, top: 519, right: 24, bottom: 500, font: ""},
		{text: "y", left: 24, top: 510, right: 34, bottom: 490, font: ""},
	}
	for i := range 20 {
		left := float64(i) * 6
		chars = append(chars, pdfChar{
			text: "e", left: left, top: 406, right: left + 5, bottom: 400, font: "",
		})
	}

	lines := groupLines(chars)
	if len(lines) != 2 {
		t.Fatalf(
			"expected 2 lines (title + body paragraph), got %d: %+v",
			len(lines),
			lines,
		)
	}

	var title *pdfLine
	for i := range lines {
		if strings.Contains(lines[i].text, "W") {
			title = &lines[i]
		}
	}
	if title == nil {
		t.Fatalf("could not find the title line among: %+v", lines)
	}
	if got, want := title.text, "Why"; got != want {
		t.Fatalf(
			"title line text = %q, want %q (title glyphs fractured into fake sub-lines)",
			got,
			want,
		)
	}
}

// TestGroupLines_SmallCharAttachesToClosestOfTwoOverlappingLines: a small
// char overlapping two envelopes joins the nearest midpoint.
func TestGroupLines_SmallCharAttachesToClosestOfTwoOverlappingLines(t *testing.T) {
	chars := []pdfChar{
		{text: "h", left: 0, top: 586.01, right: 8, bottom: 574.12, font: ""},
		{text: "i", left: 8, top: 585.77, right: 12, bottom: 574.50, font: ""},
		{text: "b", left: 0, top: 572.01, right: 8, bottom: 560.12, font: ""},
		{text: "y", left: 8, top: 566.82, right: 16, bottom: 556.26, font: ""},
		{text: "e", left: 16, top: 568.03, right: 23, bottom: 560.32, font: ""},
		// Closer to line 1's midpoint (~580.07) than line 2's (~566.07).
		{text: "\"", left: 20, top: 578, right: 23, bottom: 572, font: ""},
	}

	lines := groupLines(chars)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %+v", len(lines), lines)
	}

	var line1 *pdfLine
	for i := range lines {
		if strings.Contains(lines[i].text, "hi") {
			line1 = &lines[i]
		}
	}
	if line1 == nil {
		t.Fatalf("could not find the 'hi' line among: %+v", lines)
	}
	if !strings.Contains(line1.text, "\"") {
		t.Fatalf(
			"expected the quotation mark to attach to the closer 'hi' line, got %q",
			line1.text,
		)
	}
}

// TestRebuildHeadingLineText_MergesTrackedSmallCaps: a heading-sized line is
// re-joined with headingSpaceRatio (fixing "I ntroduction"), while a body
// line with the same gaps keeps its spaces.
func TestRebuildHeadingLineText_MergesTrackedSmallCaps(t *testing.T) {
	// Median height 10.4: the body ratio (2.6) wrongly spaces the 2.9pt letter gap.
	headingChars := []pdfChar{
		{text: "I", left: 153.5, right: 155.4, top: 110.2, bottom: 92.6, font: ""},
		{text: "n", left: 158.3, right: 167.2, top: 105.2, bottom: 94.8, font: ""},
		{text: "t", left: 168.8, right: 175.2, top: 105.2, bottom: 94.8, font: ""},
		{text: "r", left: 176.9, right: 182.3, top: 105.2, bottom: 94.8, font: ""},
		{text: "o", left: 183.3, right: 193.5, top: 105.2, bottom: 94.8, font: ""},
		{text: "d", left: 195.5, right: 205.4, top: 105.2, bottom: 94.8, font: ""},
		{text: "u", left: 208.0, right: 216.9, top: 105.2, bottom: 94.8, font: ""},
		{text: "c", left: 219.2, right: 228.1, top: 105.2, bottom: 94.8, font: ""},
		{text: "t", left: 228.8, right: 235.3, top: 105.2, bottom: 94.8, font: ""},
		{text: "i", left: 236.8, right: 239.2, top: 105.2, bottom: 94.8, font: ""},
		{text: "o", left: 241.3, right: 251.5, top: 105.2, bottom: 94.8, font: ""},
		{text: "n", left: 253.8, right: 262.7, top: 105.2, bottom: 94.8, font: ""},
		{text: "T", left: 269.3, right: 282.9, top: 110.2, bottom: 92.6, font: ""},
		{text: "i", left: 283.6, right: 286.0, top: 105.2, bottom: 94.8, font: ""},
		{text: "t", left: 287.0, right: 293.4, top: 105.2, bottom: 94.8, font: ""},
		{text: "l", left: 294.4, right: 296.8, top: 105.2, bottom: 94.8, font: ""},
		{text: "e", left: 297.8, right: 307.1, top: 105.2, bottom: 94.8, font: ""},
	}
	heading := buildLine(headingChars)
	if got, want := heading.text, "I ntroduction Title"; got != want {
		t.Fatalf("pre-rebuild heading text = %q, want %q", got, want)
	}

	bodyChars := []pdfChar{
		{text: "h", left: 100, right: 106, top: 52.5, bottom: 47.5, font: ""},
		{text: "i", left: 106.9, right: 110, top: 53.9, bottom: 51.1, font: ""},
		{text: "w", left: 114.7, right: 122.3, top: 52.4, bottom: 47.6, font: ""},
		{text: "o", left: 123.1, right: 129.1, top: 52.5, bottom: 47.5, font: ""},
		{text: "r", left: 129.9, right: 133.4, top: 52.5, bottom: 47.5, font: ""},
		{text: "d", left: 134.2, right: 139.4, top: 53.9, bottom: 51.1, font: ""},
	}
	body := buildLine(bodyChars)

	pages := []pageResult{
		{items: []streamItem{ //nolint:exhaustruct // only line matters
			{line: &heading}, //nolint:exhaustruct // line/figure union
			{line: &body},    //nolint:exhaustruct // line/figure union
		}},
	}
	rebuildHeadingLineText(pages, 5.0) // modal body height 5.0

	if got := heading.text; got != "Introduction Title" {
		t.Fatalf(
			"heading text after rebuild = %q, want %q (tracked letters must re-join)",
			got, "Introduction Title",
		)
	}
	if got, want := body.text, "hi word"; got != want {
		t.Fatalf(
			"body text after rebuild = %q, want %q (body lines untouched)",
			got,
			want,
		)
	}
}

// TestIsSmallCapsInitial covers isSmallCapsInitial's boundary cases.
func TestIsSmallCapsInitial(t *testing.T) {
	tests := map[string]struct {
		prev, c pdfChar
		want    bool
	}{
		"tall cap over small-caps continuation": {
			prev: pdfChar{text: "I", left: 0, right: 2, top: 110, bottom: 92, font: ""},
			c:    pdfChar{text: "n", left: 3, right: 9, top: 105, bottom: 95, font: ""},
			want: true, // 18 vs 10: 44% taller
		},
		"title-case same-height cap over lowercase": {
			prev: pdfChar{
				text:   "I",
				left:   0,
				right:  2,
				top:    110,
				bottom: 102,
				font:   "",
			},
			c: pdfChar{
				text:   "w",
				left:   3,
				right:  9,
				top:    110,
				bottom: 100,
				font:   "",
			},
			want: false,
		},
		"cap over capital continuation": {
			prev: pdfChar{text: "A", left: 0, right: 2, top: 110, bottom: 92, font: ""},
			c: pdfChar{
				text:   "N",
				left:   3,
				right:  9,
				top:    110,
				bottom: 102,
				font:   "",
			},
			want: false, // word boundary in an all-caps heading
		},
		"lowercase predecessor": {
			prev: pdfChar{text: "g", left: 0, right: 2, top: 110, bottom: 92, font: ""},
			c:    pdfChar{text: "m", left: 3, right: 9, top: 105, bottom: 95, font: ""},
			want: false,
		},
		"multi-char predecessor": {
			prev: pdfChar{
				text:   "TH",
				left:   0,
				right:  2,
				top:    110,
				bottom: 92,
				font:   "",
			},
			c:    pdfChar{text: "e", left: 3, right: 9, top: 105, bottom: 95, font: ""},
			want: false,
		},
		"zero-height boxes": {
			prev: pdfChar{
				text:   "I",
				left:   0,
				right:  2,
				top:    100,
				bottom: 100,
				font:   "",
			},
			c: pdfChar{
				text:   "n",
				left:   3,
				right:  9,
				top:    100,
				bottom: 100,
				font:   "",
			},
			want: false,
		},
	}
	for name, tc := range tests {
		assert.Equalf(
			t, tc.want, isSmallCapsInitial(tc.prev, tc.c),
			"isSmallCapsInitial(%+v, %+v) [%s]", tc.prev, tc.c, name,
		)
	}
}
