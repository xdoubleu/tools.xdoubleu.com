//nolint:testpackage // testing unexported service helpers
package services

import "testing"

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
