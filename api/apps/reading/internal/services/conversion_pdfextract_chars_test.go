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
