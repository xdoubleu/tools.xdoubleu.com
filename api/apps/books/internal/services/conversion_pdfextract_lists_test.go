//nolint:testpackage // testing unexported service helpers
package services

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testCharW    = 4.0
	testLineH    = 10.5
	testColLeft  = 56.0
	testColRight = 379.8
)

// lineAt builds a line of text starting at left, one testCharW-wide box per
// rune (a space advances without a box), with the column stats applied.
func lineAt(text string, left, bottom float64) pdfLine {
	var chars []pdfChar
	x := left
	for _, r := range text {
		if r != ' ' {
			chars = append(chars, pdfChar{
				text: string(r), left: x, top: bottom + 7, right: x + testCharW - 0.5,
				bottom: bottom, font: "", stream: noStreamPos,
			})
		}
		x += testCharW
	}
	l := buildLine(chars)
	l.colModalXStart, l.colRightEdge = testColLeft, testColRight
	return l
}

func TestStartsNewParagraph(t *testing.T) {
	t.Parallel()

	full := strings.Repeat("x", 80)
	cases := []struct {
		name      string
		prev, cur pdfLine
		want      bool
	}{
		{
			name: "block of indented lines (epigraph) continues",
			prev: lineAt(strings.Repeat("e", 68), 80, 100),
			cur:  lineAt("looked at in the right way"+strings.Repeat("e", 42), 80, 100-testLineH),
			want: false,
		},
		{
			name: "jump into a narrowed band beside a note continues",
			prev: lineAt(full, testColLeft, 100),
			cur:  lineAt(strings.Repeat("y", 52), 170, 100-testLineH),
			want: false,
		},
		{
			name: "first-line indent starts a paragraph",
			prev: lineAt(full, testColLeft, 100),
			cur:  lineAt("The presence of a feedback", testColLeft+9, 100-testLineH),
			want: true,
		},
		{
			name: "hanging continuation under a numbered note continues",
			prev: lineAt("1. Russell Ackoff, The Future of Operational Research Is Past, Journal of th", 62, 100),
			cur:  lineAt("Research Society 30", 62+3*testCharW, 100-testLineH),
			want: false,
		},
		{
			name: "numbered marker starts a new item",
			prev: lineAt(full, 62+3*testCharW, 100),
			cur:  lineAt("4. Honore Balzac, quoted in", 62, 100-testLineH),
			want: true,
		},
		{
			name: "text narrowed beside a figure continues",
			prev: lineAt(strings.Repeat("n", 51), 54, 100),
			cur:  lineAt(strings.Repeat("m", 51), 54, 100-testLineH),
			want: false,
		},
		{
			name: "short last line beside a figure ends the paragraph",
			prev: lineAt(strings.Repeat("n", 39), 54, 100),
			cur:  lineAt(strings.Repeat("m", 51), 54, 100-testLineH),
			want: true,
		},
		{
			name: "bullet starts a new item",
			prev: lineAt(full, 87, 100),
			cur:  lineAt("• farmers, dealers, and bankers", 87, 100-testLineH),
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rules := paraRules{
				medLineHeight: testLineH, medCharWidth: testCharW, hanging: false, index: false,
			}
			assert.Equal(t, tc.want,
				startsNewParagraph([]pdfLine{tc.prev}, tc.cur, rules))
		})
	}
}

// makeBulletListPDF: a paragraph, a bullet list whose first item wraps onto
// a hanging continuation line, then another paragraph.
func makeBulletListPDF(t *testing.T) string {
	t.Helper()
	pdf := newFixturePDF()
	cp1252 := pdf.UnicodeTranslatorFromDescriptor("")
	y := 80.0
	line := func(x float64, s string) {
		fixtureText(pdf, x, y, fixtureBodySize, cp1252(s))
		y += fixtureSameParaDY
	}
	line(fixtureLeftX, "Consider the combined purposes of the actors involved in this system:")
	y += fixtureSameParaDY
	line(fixtureLeftX+30, "• desperate people who want quick relief from psychological")
	line(fixtureLeftX+37, "pain")
	line(fixtureLeftX+30, "• farmers, dealers, and bankers who want to earn money")
	line(fixtureLeftX+30, "• wealthy people living in close proximity to poor people")
	y += fixtureSameParaDY
	line(fixtureLeftX, "Altogether, these make up a system from which it is extremely difficult")
	line(fixtureLeftX, "to eradicate drug addiction and crime. More closing words on this page.")
	return savePDF(t, pdf, "bullets.pdf")
}

func TestGoPDFConverter_BulletListBecomesUL(t *testing.T) {
	doc := string(readZipEntry(t, convertToEPUB(t, makeBulletListPDF(t))))

	ul := regexp.MustCompile(`(?s)<ul>(.*?)</ul>`).FindAllStringSubmatch(doc, -1)
	require.Len(t, ul, 1, doc)
	items := regexp.MustCompile(`<li>(.*?)</li>`).FindAllStringSubmatch(ul[0][1], -1)
	got := make([]string, len(items))
	for i, m := range items {
		got[i] = m[1]
	}
	assert.Equal(t, []string{
		"desperate people who want quick relief from psychological pain",
		"farmers, dealers, and bankers who want to earn money",
		"wealthy people living in close proximity to poor people",
	}, got)
}

// hangingLines lays out bibliography-style entries: each entry's first line
// at the margin, filling the column, its continuation lines 2 chars right.
func hangingLines(entries [][]int) []pdfLine {
	var lines []pdfLine
	y := 600.0
	for _, entry := range entries {
		for i, n := range entry {
			left := testColLeft
			if i > 0 {
				left += 2 * testCharW
			}
			lines = append(lines, lineAt(strings.Repeat("w", n), left, y))
			y -= testLineH
		}
	}
	return lines
}

func paragraphsOf(lines []pdfLine) []int {
	items := make([]streamItem, len(lines))
	for i := range lines {
		items[i] = streamItem{line: &lines[i], figure: nil, aside: nil}
	}
	blocks := buildPageBlocks(items, testLineH, testCharW)
	counts := make([]int, len(blocks))
	for i, b := range blocks {
		counts[i] = len(strings.Fields(b.text))
	}
	return counts
}

// TestBuildPageBlocks_HangingIndentEntries: in a bibliography's hanging
// layout, a stepped-right line after a full one continues its entry, and a
// line back at the margin starts the next entry.
func TestBuildPageBlocks_HangingIndentEntries(t *testing.T) {
	t.Parallel()

	lines := hangingLines([][]int{{80, 78, 30}, {80, 40}, {80, 78, 60}, {50}, {80, 20}})
	assert.Equal(t, []int{3, 2, 3, 1, 2}, paragraphsOf(lines))
}

// TestBuildPageBlocks_ProseIndentsStillBreak: first-line indents in prose
// keep starting paragraphs, full-width lines keep continuing them.
func TestBuildPageBlocks_ProseIndentsStillBreak(t *testing.T) {
	t.Parallel()

	var lines []pdfLine
	y := 600.0
	add := func(left float64, n int) {
		lines = append(lines, lineAt(strings.Repeat("w", n), left, y))
		y -= testLineH
	}
	for range 4 {
		add(testColLeft+2.5*testCharW, 78)
		add(testColLeft, 80)
		add(testColLeft, 80)
		add(testColLeft, 30)
	}
	assert.Equal(t, []int{4, 4, 4, 4}, paragraphsOf(lines))
}

// TestBuildPageBlocks_RaggedNarrowBlockKeepsLines: in a ragged-right box set
// narrower than the column, a line whose next word would fit the column
// (but not the box) still continues its paragraph.
func TestBuildPageBlocks_RaggedNarrowBlockKeepsLines(t *testing.T) {
	t.Parallel()

	var lines []pdfLine
	y := 600.0
	// Each line opens with a long word that wouldn't have fitted on the
	// line before within the box, as a ragged-right typesetter leaves it.
	for _, n := range []int{9, 10, 8, 10, 3} {
		text := "interconnections " + strings.TrimSpace(strings.Repeat("word ", n))
		lines = append(lines, lineAt(text, 79, y))
		y -= testLineH
	}
	assert.Equal(t, []int{45}, paragraphsOf(lines))
}

// TestBuildPageBlocks_CenteredHeadingEndsBeforeBody: a centred heading with
// no gap to the paragraph below still stands alone.
func TestBuildPageBlocks_CenteredHeadingEndsBeforeBody(t *testing.T) {
	t.Parallel()

	heading := lineAt("THE TRAP POLICY RESISTANCE", 132, 600)
	body := []pdfLine{
		lineAt(strings.Repeat("w", 80), testColLeft, 600-testLineH),
		lineAt(strings.Repeat("w", 30), testColLeft, 600-2*testLineH),
	}
	assert.Equal(t, []int{4, 2}, paragraphsOf(append([]pdfLine{heading}, body...)))
}

// TestBuildPageBlocks_IndexEntriesStaySeparate: index lines ending in page
// references are entries of their own, even when the next entry's first word
// wouldn't have fitted after them; a stepped-right line continues an entry.
func TestBuildPageBlocks_IndexEntriesStaySeparate(t *testing.T) {
	t.Parallel()

	var lines []pdfLine
	y := 600.0
	add := func(left float64, text string) {
		l := lineAt(text, left, y)
		l.colRightEdge = testColLeft + 44*testCharW
		lines = append(lines, l)
		y -= testLineH
	}
	add(testColLeft, "archetypes")
	add(testColLeft+3*testCharW, "addiction archetype, 131–135, 193")
	add(testColLeft+3*testCharW, "commons system archetype, tragedy of the")
	add(testColLeft+5*testCharW, "116–121, 191–192")
	add(testColLeft+3*testCharW, "eroding goals archetype, 121–123, 190")
	add(testColLeft+3*testCharW, "escalation archetype, 124–126, 192")
	add(testColLeft, "Bateson, Gregory, ix")
	assert.Equal(t, []int{1, 4, 8, 5, 4, 3}, paragraphsOf(lines))
}
