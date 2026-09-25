//nolint:testpackage // testing unexported service helpers
package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func textBlock(text string, medHeight float64) htmlBlock {
	return htmlBlock{ //nolint:exhaustruct // html/tag filled in by finalizeHeadings
		text:      text,
		medHeight: medHeight,
		isText:    true,
	}
}

func imgBlockForTest() htmlBlock {
	return htmlBlock{ //nolint:exhaustruct // only tag/isText matter here
		tag: imgTag,
	}
}

// TestFinalizeHeadings_DemotesRunOfLargeFontCitations: a run of large-font
// citation lines stays "p"; only the isolated real heading is promoted.
func TestFinalizeHeadings_DemotesRunOfLargeFontCitations(t *testing.T) {
	const modal = 10.0

	blocks := []htmlBlock{
		textBlock("Chapter One: Systems Thinking", modal*1.5), // real heading
		textBlock("Body paragraph one of the chapter.", modal),
		textBlock("Body paragraph two of the chapter.", modal),
		textBlock(
			"Other Books by the Author:",
			modal*1.5,
		), // frontmatter section title
		textBlock(
			"The Global Citizen (1991).",
			modal*1.5,
		), // citation 1 (h1-sized)
		textBlock(
			"Limits to Growth: The 30-Year Update.",
			modal*1.2,
		), // citation 2 (h2-sized)
		textBlock("Thinking in Systems (2008).", modal*1.5), // citation 3
	}

	finalizeHeadings(blocks, modal)

	assert.Equal(
		t,
		"h1",
		blocks[0].tag,
		"isolated large-font heading must stay a heading",
	)
	assert.Contains(t, blocks[0].html, "<h1>")

	assert.Equal(t, "p", blocks[1].tag)
	assert.Equal(t, "p", blocks[2].tag)

	for i := 3; i < len(blocks); i++ {
		assert.Equalf(
			t,
			"p",
			blocks[i].tag,
			"block %d (%q) is part of a citation run and must not become a heading",
			i,
			blocks[i].text,
		)
	}
}

// TestFinalizeHeadings_ImageBreaksRun: a figure splits a run.
func TestFinalizeHeadings_ImageBreaksRun(t *testing.T) {
	const modal = 10.0

	blocks := []htmlBlock{
		textBlock("A Real Heading", modal*1.5),
		imgBlockForTest(),
		textBlock("Body paragraph after the figure.", modal),
	}

	finalizeHeadings(blocks, modal)

	assert.Equal(t, "h1", blocks[0].tag)
	assert.Equal(t, "p", blocks[2].tag)
}

// TestFinalizeHeadings_ZeroModalHeight: no body baseline yields plain paragraphs.
func TestFinalizeHeadings_ZeroModalHeight(t *testing.T) {
	blocks := []htmlBlock{textBlock("Some text", 12)}
	finalizeHeadings(blocks, 0)
	assert.Equal(t, "p", blocks[0].tag)
}

// TestFinalizeHeadings_DemotesLowercaseStartCandidate: a lowercase-start
// candidate is demoted.
func TestFinalizeHeadings_DemotesLowercaseStartCandidate(t *testing.T) {
	const modal = 10.0

	blocks := []htmlBlock{
		textBlock("Chapter One: Systems Thinking", modal*1.5), // real heading
		textBlock("Body paragraph one of the chapter.", modal),
		textBlock(
			"reinforcing loop", // marginal pull-quote fragment
			modal*1.5,
		),
		textBlock("Body paragraph two of the chapter.", modal),
		textBlock(
			"which is why the system oscillates", // run-on fragment
			modal*1.2,
		),
	}

	finalizeHeadings(blocks, modal)

	assert.Equal(t, "h1", blocks[0].tag, "real heading must stay a heading")
	assert.Equal(t, "p", blocks[2].tag, "lowercase-start candidate must be demoted")
	assert.Equal(t, "p", blocks[4].tag, "lowercase-start candidate must be demoted")
}

// TestFinalizeHeadings_DemotesLoopDiagramLabel: isolated all-caps loop labels
// ("R B") are demoted.
func TestFinalizeHeadings_DemotesLoopDiagramLabel(t *testing.T) {
	const modal = 10.0

	blocks := []htmlBlock{
		textBlock("Chapter One: Systems Thinking", modal*1.5), // real heading
		textBlock("Body paragraph one of the chapter.", modal),
		textBlock("B", modal*1.5),   // single balancing-loop label
		textBlock("R B", modal*1.5), // reinforcing + balancing loop labels
		textBlock("B B", modal*1.5), // two balancing-loop labels
		textBlock("Body paragraph two of the chapter.", modal),
		// Title-page size: a mid-band one-worder would be demoted as a figure label.
		textBlock("Appendix", modal*3.0),
	}

	finalizeHeadings(blocks, modal)

	assert.Equal(t, "h1", blocks[0].tag, "real heading must stay a heading")
	assert.Equal(t, "p", blocks[2].tag, `"B" loop label must be demoted`)
	assert.Equal(t, "p", blocks[3].tag, `"R B" loop label must be demoted`)
	assert.Equal(t, "p", blocks[4].tag, `"B B" loop label must be demoted`)
	assert.Equal(
		t,
		"h1",
		blocks[6].tag,
		"a real short all-caps heading must not be demoted",
	)
}

// TestFinalizeHeadings_KeepsTwoLineTitlePair: a same-size two-line title with
// no list shape stays h1.
func TestFinalizeHeadings_KeepsTwoLineTitlePair(t *testing.T) {
	const modal = 10.0

	blocks := []htmlBlock{
		textBlock("— SIX —", modal*1.32),
		textBlock("Leverage Points—", modal*2.09),
		textBlock("Places to I ntervene in a System", modal*2.09),
		textBlock("Body paragraph after the title page.", modal),
	}

	finalizeHeadings(blocks, modal)

	assert.Equal(t, "h1", blocks[1].tag)
	assert.Equal(t, "h1", blocks[2].tag)
}

// TestFinalizeHeadings_KeepsEllipsisTitle: a trailing ". . ." isn't sentence
// punctuation.
func TestFinalizeHeadings_KeepsEllipsisTitle(t *testing.T) {
	const modal = 10.0

	blocks := []htmlBlock{
		textBlock("System Traps . . .", modal*2.09),
		textBlock("Body paragraph.", modal),
	}

	finalizeHeadings(blocks, modal)

	assert.Equal(t, "h1", blocks[0].tag)
}

// TestIsLoopDiagramLabel covers isLoopDiagramLabel's boundary cases.
func TestIsLoopDiagramLabel(t *testing.T) {
	tests := map[string]bool{
		"B":        true,
		"R B":      true,
		"B B":      true,
		"R":        true,
		"":         false,
		"Appendix": false,
		"NOTES":    false,
		"BB":       false, // not space-separated single-letter tokens
		"b b":      false, // lowercase
		"B b":      false, // mixed case
		" B ":      true,  // surrounding whitespace is trimmed
	}
	for text, want := range tests {
		assert.Equalf(
			t, want, isLoopDiagramLabel(text), "isLoopDiagramLabel(%q)", text,
		)
	}
}

// TestFinalizeHeadings_KeepsChapterTitleBesideBannerLine: a title beside its
// dissimilar-size banner survives.
func TestFinalizeHeadings_KeepsChapterTitleBesideBannerLine(t *testing.T) {
	const modal = 10.0

	blocks := []htmlBlock{
		textBlock("— ONE —", modal*1.32),    // banner line (h2-sized candidate)
		textBlock("The Basics", modal*2.09), // real chapter title
		textBlock("Body paragraph after the title page.", modal),
	}

	finalizeHeadings(blocks, modal)

	assert.Equal(
		t, "h1", blocks[1].tag,
		"chapter title beside a differently-sized banner line must stay h1",
	)
}

// TestFinalizeHeadings_DemotesRunOfThreeSimilarHeights: 3+ similar-size
// candidates are a list regardless of shape.
func TestFinalizeHeadings_DemotesRunOfThreeSimilarHeights(t *testing.T) {
	const modal = 10.0

	blocks := []htmlBlock{
		textBlock("Real Heading", modal*1.5),
		textBlock("Body paragraph.", modal),
		textBlock("Entry one", modal*1.5),
		textBlock("Entry two", modal*1.45),
		textBlock("Entry three", modal*1.5),
	}

	finalizeHeadings(blocks, modal)

	assert.Equal(t, "h1", blocks[0].tag)
	for i := 2; i < len(blocks); i++ {
		assert.Equalf(t, "p", blocks[i].tag,
			"block %d (%q) is part of a 3+ candidate run", i, blocks[i].text)
	}
}

// TestFinalizeHeadings_KeepsIsolatedDissimilarCandidates: a size jump splits
// the chain.
func TestFinalizeHeadings_KeepsIsolatedDissimilarCandidates(t *testing.T) {
	const modal = 10.0

	blocks := []htmlBlock{
		textBlock("A Real Heading", modal*1.5),
		textBlock("Another One-Word Label", modal*1.2),
	}

	finalizeHeadings(blocks, modal)

	assert.Equal(t, "h1", blocks[0].tag)
	assert.Equal(t, "h2", blocks[1].tag)
}

// TestFinalizeHeadings_DemotesPairOfSimilarLargeCandidates covers the
// two-member edge.
func TestFinalizeHeadings_DemotesPairOfSimilarLargeCandidates(t *testing.T) {
	const modal = 10.0

	blocks := []htmlBlock{
		textBlock("Body paragraph.", modal),
		textBlock("Citation one.", modal*1.5),
		textBlock("Citation two.", modal*1.45),
		textBlock("Body paragraph.", modal),
		textBlock("— TWO —", modal*1.32),
		textBlock("A Brief Visit to the Systems Zoo", modal*2.71),
	}

	finalizeHeadings(blocks, modal)

	assert.Equal(t, "p", blocks[1].tag)
	assert.Equal(t, "p", blocks[2].tag, "similar-size pair must be demoted")
	assert.Equal(
		t, "h1", blocks[5].tag,
		"dissimilar-size pair (banner + chapter title) must keep the title",
	)
}

// TestFinalizeHeadings_DemotesSentencePunctuationHeading: sentence-shaped
// candidates (incl. "13. …", "Why?") are demoted.
func TestFinalizeHeadings_DemotesSentencePunctuationHeading(t *testing.T) {
	const modal = 10.0

	blocks := []htmlBlock{
		textBlock("THE WAY OUT", modal*1.61), // real small-caps heading
		textBlock("Body paragraph.", modal),
		textBlock(
			"The one who had felt its feet and legs said: "+
				"“It is mighty and firm, like a pillar.”",
			modal*1.5,
		),
		textBlock("13. Defy the disciplines.", modal*1.42), // numbered list item
		textBlock("Why?", modal*1.5),                       // question fragment
		textBlock("Body paragraph.", modal),
		textBlock("Appendix", modal*3.09), // real heading
	}

	finalizeHeadings(blocks, modal)

	assert.Equal(t, "h1", blocks[0].tag)
	assert.Equal(t, "p", blocks[2].tag, "quoted sentence must be demoted")
	assert.Equal(t, "p", blocks[3].tag, "numbered list item must be demoted")
	assert.Equal(t, "p", blocks[4].tag, "question fragment must be demoted")
	assert.Equal(t, "h1", blocks[6].tag)
}

// TestFinalizeHeadings_DemotesFigureReferenceHeading: figure references are
// captions, not headings.
func TestFinalizeHeadings_DemotesFigureReferenceHeading(t *testing.T) {
	const modal = 10.0

	blocks := []htmlBlock{
		textBlock("Delays, Figure 30:", modal*1.42),
		textBlock("(see Figure 39).", modal*1.5),
		textBlock("Body paragraph.", modal),
		textBlock("Stabilizing Loops—Balancing Feedback", modal*1.54),
	}

	finalizeHeadings(blocks, modal)

	assert.Equal(t, "p", blocks[0].tag)
	assert.Equal(t, "p", blocks[1].tag)
	assert.Equal(t, "h1", blocks[3].tag)
}

// TestFinalizeHeadings_DemotesSingleWordMidBandHeading: a mid-band one-word h1
// is a figure label.
func TestFinalizeHeadings_DemotesSingleWordMidBandHeading(t *testing.T) {
	const modal = 10.0

	blocks := []htmlBlock{
		textBlock("Cooling", modal*1.49),
		textBlock("Body paragraph.", modal),
		textBlock("Appendix", modal*3.09), // real one-word heading, title-page size
	}

	finalizeHeadings(blocks, modal)

	assert.Equal(t, "p", blocks[0].tag)
	assert.Equal(t, "h1", blocks[2].tag)
}
