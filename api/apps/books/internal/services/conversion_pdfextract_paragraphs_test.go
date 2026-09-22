//nolint:testpackage // testing unexported service helpers
package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// textBlock builds a minimal paragraph htmlBlock for finalizeHeadings tests,
// bypassing PDF extraction entirely.
func textBlock(text string, medHeight float64) htmlBlock {
	return htmlBlock{ //nolint:exhaustruct // html/tag filled in by finalizeHeadings
		text:      text,
		medHeight: medHeight,
		isText:    true,
	}
}

// imgBlockForTest builds a minimal image htmlBlock (isText: false), used to
// verify that a figure breaks a run of large-font text blocks.
func imgBlockForTest() htmlBlock {
	return htmlBlock{ //nolint:exhaustruct // only tag/isText matter here
		tag: imgTag,
	}
}

// TestFinalizeHeadings_DemotesRunOfLargeFontCitations reproduces issue
// #1654's bibliography over-fire: a document with one real, isolated
// large-font chapter heading plus a run of several large-font
// bibliography-style citation lines (e.g. a frontmatter "Other books by this
// author" list) must classify only the real heading as h1/h2 — the citation
// run, despite exceeding the same height ratio, must stay plain paragraphs.
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

// TestFinalizeHeadings_ImageBreaksRun verifies that a figure between two
// large-font paragraphs prevents them from being treated as one run, so each
// is judged in isolation.
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

// TestFinalizeHeadings_ZeroModalHeight verifies the no-body-text-baseline
// edge case (e.g. an all-heading-sized page) falls back to plain paragraphs
// rather than dividing by zero.
func TestFinalizeHeadings_ZeroModalHeight(t *testing.T) {
	blocks := []htmlBlock{textBlock("Some text", 12)}
	finalizeHeadings(blocks, 0)
	assert.Equal(t, "p", blocks[0].tag)
}

// TestFinalizeHeadings_DemotesLowercaseStartCandidate reproduces issue
// #1698's marginal-pull-quote/mid-sentence false positives: a height-ratio
// heading candidate whose text starts with a lowercase letter (never true of
// a real title) must be demoted to "p", while an isolated, properly
// capitalized heading of the same size stays a heading.
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

// TestFinalizeHeadings_DemotesLoopDiagramLabel reproduces the follow-up
// #1766 explicitly left open: a handful of very short all-caps
// systems-diagram loop labels ("B", "R B", "B B") from the reference book
// ("Thinking in Systems") are large enough to clear the heading height
// ratio, isolated (so demoteHeadingRuns's run-of-2+ guard never fires), and
// all-uppercase (so startsLowercase never fires either) — yet they are not
// real headings and must not pollute the generated EPUB chapter TOC.
func TestFinalizeHeadings_DemotesLoopDiagramLabel(t *testing.T) {
	const modal = 10.0

	blocks := []htmlBlock{
		textBlock("Chapter One: Systems Thinking", modal*1.5), // real heading
		textBlock("Body paragraph one of the chapter.", modal),
		textBlock("B", modal*1.5),   // single balancing-loop label
		textBlock("R B", modal*1.5), // reinforcing + balancing loop labels
		textBlock("B B", modal*1.5), // two balancing-loop labels
		textBlock("Body paragraph two of the chapter.", modal),
		// A real short all-caps heading stays a heading. It's sized like a
		// title-page heading (3x body): a one-worder at mid-band size is an
		// embedded figure label and is demoted — see
		// TestFinalizeHeadings_DemotesSingleWordMidBandHeading (issue #1698).
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

// TestFinalizeHeadings_KeepsTwoLineTitlePair reproduces the reopened issue
// #1698's "Leverage Points—" chapter: its title wraps onto two lines at the
// same title-page size. A similar-size pair only counts as list entries
// when a member has an entry's text shape (punctuation, figure reference,
// …); a two-line title has none and both lines must keep the title as h1.
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

// TestFinalizeHeadings_KeepsEllipsisTitle verifies a title ending in a
// spaced typographic ellipsis ("System Traps . . .") is not treated as a
// sentence (the trailing dots are the book's ellipsis, not sentence-ending
// punctuation) and stays h1.
func TestFinalizeHeadings_KeepsEllipsisTitle(t *testing.T) {
	const modal = 10.0

	blocks := []htmlBlock{
		textBlock("System Traps . . .", modal*2.09),
		textBlock("Body paragraph.", modal),
	}

	finalizeHeadings(blocks, modal)

	assert.Equal(t, "h1", blocks[0].tag)
}

// TestIsLoopDiagramLabel covers isLoopDiagramLabel's boundary cases directly:
// single/multi single-letter tokens match, while real words (even short
// all-caps ones) and lowercase/mixed-case text don't.
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

// TestFinalizeHeadings_KeepsChapterTitleBesideBannerLine reproduces the
// reopened issue #1698's missing-chapters half: on a chapter title page the
// big title ("The Basics", ~2× body height) sits directly beside its
// decoration banner ("— ONE —", ~1.3× body height, an h2-sized candidate).
// The run guard must not flatten such a dissimilar pair — only adjacent
// candidates rendered at the same size (list/citation entries) look like a
// list — so the title survives as the TOC's chapter entry.
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

// TestFinalizeHeadings_DemotesRunOfThreeSimilarHeights verifies the #1654
// bibliography guard: three consecutive large-font entries at a similar
// size are a list regardless of their text shapes — none ends in sentence
// punctuation, yet all must stay plain paragraphs.
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

// TestFinalizeHeadings_KeepsIsolatedDissimilarCandidates verifies that a
// size jump between consecutive candidates splits the chain: each side is
// judged in isolation, so two one-off large lines at different sizes stay
// headings (this is what a chapter title page's banner + title look like).
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

// TestFinalizeHeadings_DemotesPairOfSimilarLargeCandidates verifies the
// two-member edge: two consecutive candidates at the same (similar) size are
// a list fragment and are demoted, while two consecutive candidates at
// clearly different sizes are not.
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

// TestFinalizeHeadings_DemotesSentencePunctuationHeading reproduces the
// reopened issue #1698's pull-quote/fragment TOC entries: an h1-sized
// candidate ending in sentence punctuation (after trimming closing quotes)
// is a sentence, never a heading. Numbered-list items ("13. …") and short
// questions ("Why?") share the shape.
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

// TestFinalizeHeadings_DemotesFigureReferenceHeading reproduces the
// reopened issue #1698's caption/cross-reference fragments ("Delays,
// Figure 30:", "(see Figure 39)."): an h1-sized candidate referencing a
// figure number is part of a caption, not a heading.
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

// TestFinalizeHeadings_DemotesSingleWordMidBandHeading reproduces the
// reopened issue #1698's diagram-label fragments ("Cooling"): a one-word h1
// candidate at mid-band size (between body text and real title-page type)
// is an embedded figure label, not a heading; real one-word headings appear
// only at title-page size.
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
