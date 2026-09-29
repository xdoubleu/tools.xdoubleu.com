//nolint:testpackage // testing unexported service helpers
package services

import (
	"testing"

	"github.com/klippa-app/go-pdfium/responses"
	"github.com/stretchr/testify/assert"
)

// structuredChar builds one PDFium structured-text char for extractChars.
func structuredChar(
	text string, left, right, bottom, top float64,
) *responses.GetPageTextStructuredChar {
	return &responses.GetPageTextStructuredChar{
		Text:  text,
		Angle: 0,
		PointPosition: responses.CharPosition{
			Left: left, Top: top, Right: right, Bottom: bottom,
		},
		PixelPosition:   nil,
		FontInformation: nil,
	}
}

func structuredPage(
	chars ...*responses.GetPageTextStructuredChar,
) *responses.GetPageTextStructured {
	return &responses.GetPageTextStructured{
		Page: 0, Chars: chars, Rects: nil, PointToPixelRatio: 0,
	}
}

// TestBuildLine_SpacesFollowPDFiumWhitespace: glyph gaps inside "generally"
// (side-bearings of a sans "l", from the reference book) exceed the geometric
// threshold, yet PDFium's stream has no space there — only before "used".
func TestBuildLine_SpacesFollowPDFiumWhitespace(t *testing.T) {
	resp := structuredPage(
		structuredChar("g", 243.0, 247.5, 639.0, 646.1),
		structuredChar("e", 248.6, 252.8, 641.4, 646.1),
		structuredChar("n", 253.8, 257.9, 641.4, 646.1),
		structuredChar("e", 258.9, 263.2, 641.4, 646.1),
		structuredChar("r", 264.1, 266.6, 641.4, 646.1),
		structuredChar("a", 267.0, 270.9, 641.4, 646.1),
		structuredChar("l", 272.1, 273.0, 641.4, 649.5),
		structuredChar("l", 274.3, 275.2, 641.4, 649.5),
		structuredChar("y", 275.9, 280.6, 639.1, 646.1),
		structuredChar(" ", 280.6, 283.1, 641.4, 641.4),
		structuredChar("u", 283.7, 287.8, 641.4, 646.1),
		structuredChar("s", 288.7, 291.9, 641.4, 646.1),
	)

	line := buildLine(extractChars(resp))
	assert.Equal(t, "generally us", line.text)
}

// TestBuildLine_SpaceGlyphWithoutGeometricGap: an overhanging "f" leaves no
// visible gap before the next word, but PDFium reports the space glyph.
func TestBuildLine_SpaceGlyphWithoutGeometricGap(t *testing.T) {
	resp := structuredPage(
		structuredChar("o", 363.5, 368.3, 658.4, 663.4),
		structuredChar("f", 369.0, 373.1, 658.5, 666.3),
		structuredChar(" ", 372.3, 374.8, 658.5, 658.5),
		structuredChar("t", 374.0, 378.7, 658.4, 664.7),
		structuredChar("h", 379.0, 384.4, 658.5, 666.3),
	)

	line := buildLine(extractChars(resp))
	assert.Equal(t, "of th", line.text)
}

// TestBuildLine_LineBreakFallsBackToGeometry: across a PDFium line break the
// stream says nothing about word spacing, so the gap check decides.
func TestBuildLine_LineBreakFallsBackToGeometry(t *testing.T) {
	resp := structuredPage(
		structuredChar("a", 0, 5, 100, 107),
		structuredChar("\r", 5, 5, 100, 100),
		structuredChar("\n", 5, 5, 100, 100),
		structuredChar("1", 5.2, 8, 103, 108),
		structuredChar("\r", 8, 8, 100, 100),
		structuredChar("\n", 8, 8, 100, 100),
		structuredChar("b", 20, 25, 100, 107),
	)

	chars := extractChars(resp)
	assert.Equal(t, "a1 b", joinChars(chars, lineSpaceGapRatio))
}

// TestBuildLine_LigatureSpaceGlyphIgnored: InDesign follows every ligature
// with a space glyph ("fl ows" in the reference book); PDFium decomposes the
// ligature into letters sharing one box, and the next letter sits a letter
// gap away.
func TestBuildLine_LigatureSpaceGlyphIgnored(t *testing.T) {
	resp := structuredPage(
		structuredChar("f", 230.4, 235.9, 571.5, 579.3),
		structuredChar("l", 230.4, 235.9, 571.5, 579.3),
		structuredChar(" ", 233.0, 235.6, 571.5, 571.5),
		structuredChar("o", 236.5, 241.3, 571.4, 576.4),
		structuredChar("w", 241.6, 249.2, 571.4, 576.3),
	)

	line := buildLine(extractChars(resp))
	assert.Equal(t, "flow", line.text)
}

// TestBuildLine_WideGapWithoutSpaceGlyph: text positioned apart with no space
// glyph between (a tab stop in a proof slug) still reads as two words.
func TestBuildLine_WideGapWithoutSpaceGlyph(t *testing.T) {
	resp := structuredPage(
		structuredChar("i", 100, 102, 50, 57),
		structuredChar("x", 102.5, 107, 50, 55),
		structuredChar("5", 130, 135, 50, 57),
	)

	line := buildLine(extractChars(resp))
	assert.Equal(t, "ix 5", line.text)
}

// TestBuildLine_WordEndingInLigatureKeepsSpace: a ligature ending a word
// ("staff meeting") is followed by a real, word-sized gap.
func TestBuildLine_WordEndingInLigatureKeepsSpace(t *testing.T) {
	resp := structuredPage(
		structuredChar("a", 225.0, 229.5, 571.5, 576.4),
		structuredChar("f", 230.4, 235.9, 571.5, 579.3),
		structuredChar("f", 230.4, 235.9, 571.5, 579.3),
		structuredChar(" ", 233.0, 235.6, 571.5, 571.5),
		structuredChar("m", 238.5, 246.0, 571.4, 576.4),
		structuredChar("e", 246.6, 250.5, 571.4, 576.4),
		structuredChar("e", 251.1, 255.0, 571.4, 576.4),
	)

	line := buildLine(extractChars(resp))
	assert.Equal(t, "aff mee", line.text)
}

// TestBuildLine_LigatureSpaceBeforeSideBearing: a letter with a wide
// side-bearing after a ligature ("fi ll" in a sans margin note) is still
// mid-word.
func TestBuildLine_LigatureSpaceBeforeSideBearing(t *testing.T) {
	resp := structuredPage(
		structuredChar("f", 140.8, 144.7, 418.5, 425.1),
		structuredChar("i", 140.8, 144.7, 418.5, 425.1),
		structuredChar(" ", 142.9, 144.9, 418.5, 418.5),
		structuredChar("l", 146.0, 147.0, 418.5, 425.1),
		structuredChar("l", 148.2, 149.2, 418.5, 425.1),
		structuredChar("s", 150.0, 153.4, 418.4, 423.3),
		structuredChar(" ", 153.4, 155.4, 418.5, 418.5),
		structuredChar("u", 156.0, 159.8, 418.4, 423.3),
		structuredChar("s", 160.4, 163.8, 418.4, 423.3),
		structuredChar("e", 164.4, 168.2, 418.4, 423.3),
		structuredChar("s", 168.8, 172.2, 418.4, 423.3),
	)

	line := buildLine(extractChars(resp))
	assert.Equal(t, "fills uses", line.text)
}

// TestGroupLines_HighApostropheStaysInStreamRun: an apostrophe set above the
// x-height of the letter before it still continues the word it's part of.
func TestGroupLines_HighApostropheStaysInStreamRun(t *testing.T) {
	resp := structuredPage(
		structuredChar("T", 100.0, 104.6, 50.0, 57.0),
		structuredChar("h", 104.8, 109.0, 50.0, 57.2),
		structuredChar("e", 109.4, 113.2, 49.9, 54.8),
		structuredChar("r", 113.8, 116.3, 50.0, 54.8),
		structuredChar("e", 116.6, 120.4, 49.9, 54.8),
		structuredChar("’", 120.8, 122.0, 55.2, 57.2),
		structuredChar("s", 122.4, 125.8, 49.9, 54.8),
		// A body line beside the note, on a baseline nearer the apostrophe.
		structuredChar("\r", 126, 126, 50, 50),
		structuredChar("\n", 126, 126, 50, 50),
		structuredChar("b", 200.0, 204.0, 52.5, 59.7),
		structuredChar("o", 204.4, 208.2, 52.4, 57.3),
		structuredChar("d", 208.6, 212.4, 52.4, 59.7),
		structuredChar("y", 212.8, 216.4, 50.1, 57.3),
	)

	lines := groupLines(extractChars(resp))
	texts := make([]string, len(lines))
	for i, l := range lines {
		texts[i] = l.text
	}
	assert.Equal(t, []string{"body", "There’s"}, texts)
}
