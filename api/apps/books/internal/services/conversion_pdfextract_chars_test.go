//nolint:testpackage // testing unexported service helpers
package services

import (
	"strings"
	"testing"
)

// TestGroupLines_CommaStaysOnLine reproduces issue #594: a comma's bounding
// box is much shorter than the surrounding letters and sits low, near/below
// the baseline (as PDFium reports it for many fonts). Clustering lines by
// box midpoint pushes the comma's yMid far enough from the line's running
// average to split it into its own one-character line, which then renders
// as a stray "," when the paragraph is rejoined. Geometry below mirrors
// real values measured from a production PDF (issue #594).
func TestGroupLines_CommaStaysOnLine(t *testing.T) {
	chars := []pdfChar{
		{text: "h", left: 0, top: 584.07, right: 8, bottom: 574.36},
		{text: "i", left: 8, top: 585.77, right: 12, bottom: 574.50},
		{text: ",", left: 12, top: 576.36, right: 15, bottom: 571.92},
		{text: "b", left: 18, top: 586.01, right: 26, bottom: 574.12},
		{text: "y", left: 26, top: 580.82, right: 34, bottom: 570.26},
		{text: "e", left: 34, top: 582.03, right: 41, bottom: 574.32},
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

// TestGroupLines_ApostropheStaysOnLine reproduces issue #618: an apostrophe's
// bounding box is much shorter than the surrounding letters and sits high,
// near cap-height rather than the baseline — the opposite offset from a
// comma. The baseline-clustering fix for #594 compares each character's
// bottom edge against the line's running average bottom, which fixed
// low-hanging commas/descenders but still pushes a high-set apostrophe's
// bottom far enough from that average to split it into its own
// one-character line. Geometry below mirrors real values measured from a
// production PDF (issue #618).
func TestGroupLines_ApostropheStaysOnLine(t *testing.T) {
	chars := []pdfChar{
		{text: "k", left: 0, top: 586.01, right: 8, bottom: 574.12},
		{text: "i", left: 8, top: 585.77, right: 12, bottom: 574.50},
		{text: "d", left: 12, top: 586.01, right: 20, bottom: 574.12},
		{text: "s", left: 20, top: 582.03, right: 27, bottom: 574.32},
		{text: "’", left: 27, top: 586.01, right: 30, bottom: 580.20},
		{text: "t", left: 33, top: 584.07, right: 38, bottom: 574.36},
		{text: "o", left: 38, top: 582.03, right: 46, bottom: 574.32},
		{text: "y", left: 46, top: 580.82, right: 54, bottom: 570.26},
		{text: "s", left: 54, top: 582.03, right: 61, bottom: 574.32},
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

// TestGroupLines_AllZeroHeightChars_FallsBackToNormalClustering covers the
// degenerate-page branch in groupLines: if every character reports zero
// height (top == bottom — malformed PDFium data), medianCharHeight is 0, so
// medH is coerced to 1 and every character then fails the
// normalCharHeightRatio check (0 >= 0.7*1 is false). groupLines falls back
// to clustering every character as if it were "normal" rather than treating
// a whole page as unattachable small punctuation.
func TestGroupLines_AllZeroHeightChars_FallsBackToNormalClustering(t *testing.T) {
	chars := []pdfChar{
		{text: "a", left: 0, top: 580, right: 5, bottom: 580},
		{text: "b", left: 5, top: 580, right: 10, bottom: 580},
	}

	lines := groupLines(chars)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %+v", len(lines), lines)
	}
	if got, want := lines[0].text, "ab"; got != want {
		t.Fatalf("line text = %q, want %q", got, want)
	}
}

// TestGroupLines_SmallCharFarFromAnyLine_StartsOwnLine covers the fallback
// branch in attachSmallChars: a short-box character with no line within
// lineGroupYMidRatio * medH starts its own single-character line instead of
// being force-attached to a distant, unrelated line.
func TestGroupLines_SmallCharFarFromAnyLine_StartsOwnLine(t *testing.T) {
	chars := []pdfChar{
		{text: "h", left: 0, top: 584.07, right: 8, bottom: 574.36},
		{text: "i", left: 8, top: 585.77, right: 12, bottom: 574.50},
		// Isolated comma far below the "hi" line — more than
		// lineGroupYMidRatio * medH away from it.
		{text: ",", left: 0, top: 400, right: 3, bottom: 396},
	}

	lines := groupLines(chars)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %+v", len(lines), lines)
	}
}

// TestGroupLines_QuotationMarksStayOnLine reproduces the broader #618 symptom
// beyond a lone apostrophe: dialogue punctuation (straight double quotes)
// bracketing a line of text, each sitting high near cap-height like the
// apostrophe case. Both quote glyphs must attach to the surrounding line by
// envelope overlap rather than splitting off into their own one-character
// lines/paragraphs.
func TestGroupLines_QuotationMarksStayOnLine(t *testing.T) {
	chars := []pdfChar{
		{text: "\"", left: 0, top: 586.01, right: 3, bottom: 580.20},
		{text: "h", left: 3, top: 584.07, right: 11, bottom: 574.36},
		{text: "i", left: 11, top: 585.77, right: 15, bottom: 574.50},
		{text: "b", left: 18, top: 586.01, right: 26, bottom: 574.12},
		{text: "y", left: 26, top: 580.82, right: 34, bottom: 570.26},
		{text: "e", left: 34, top: 582.03, right: 41, bottom: 574.32},
		{text: "\"", left: 41, top: 586.01, right: 44, bottom: 580.20},
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

// TestGroupLines_SmallCharAttachesToClosestOfTwoOverlappingLines covers the
// tie-break branch in attachSmallChars: when a small character's box
// overlaps more than one line's envelope (two lines close enough together
// that their margins both reach it), it must join whichever line's
// y-midpoint is nearest, not simply the first or last candidate considered.
func TestGroupLines_SmallCharAttachesToClosestOfTwoOverlappingLines(t *testing.T) {
	chars := []pdfChar{
		// Line 1 ("hi"), a normal line near the top.
		{text: "h", left: 0, top: 586.01, right: 8, bottom: 574.12},
		{text: "i", left: 8, top: 585.77, right: 12, bottom: 574.50},
		// Line 2 ("bye"), a second normal line close beneath line 1 — close
		// enough that a small character between them overlaps both
		// envelopes within lineGroupYMidRatio * medH.
		{text: "b", left: 0, top: 572.01, right: 8, bottom: 560.12},
		{text: "y", left: 8, top: 566.82, right: 16, bottom: 556.26},
		{text: "e", left: 16, top: 568.03, right: 23, bottom: 560.32},
		// A stray quotation mark positioned closer to line 1's y-midpoint
		// (~580.07) than line 2's (~566.07) — it must attach to line 1.
		{text: "\"", left: 20, top: 578, right: 23, bottom: 572},
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
