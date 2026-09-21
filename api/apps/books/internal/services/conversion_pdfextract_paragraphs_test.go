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
		textBlock("Appendix", modal*1.5), // real short heading stays a heading
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
