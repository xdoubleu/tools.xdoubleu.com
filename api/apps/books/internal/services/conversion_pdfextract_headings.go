package services

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// finalizeHeadings assigns each paragraph block's final tag ("h1"/"h2"/"p")
// and renders its html, in place. A paragraph is classified purely by height
// ratio to the document's modal (body text) character height, as before.
// A candidate that passes the ratio test is then demoted to "p" when its
// shape or context says it isn't a real heading:
//
//   - startsLowercase: a real title never starts mid-sentence — catches
//     large-font marginal pull-quote words and run-on paragraph fragments
//     (issue #1698).
//   - isLoopDiagramLabel: text made up only of single uppercase-letter
//     tokens ("B", "R B", "B B") is a systems-diagram loop-label callout,
//     never a title (issue #1698 follow-up left open by #1766).
//   - startsLowercase: a real title never starts mid-sentence — catches
//     large-font marginal pull-quote words and run-on paragraph fragments
//     (issue #1698).
//   - isLoopDiagramLabel: text made up only of single uppercase-letter
//     tokens ("B", "R B", "B B") is a systems-diagram loop-label callout,
//     never a title (issue #1698 follow-up left open by #1766).
//   - isHeadingFalsePositive: a candidate ending in sentence punctuation
//     (a quoted pull quote, a numbered list item, a question fragment) or
//     referencing a figure number ("Delays, Figure 30:", "(see Figure 39)")
//     is body text or a caption; an h1 candidate that is a single word at
//     mid-band size (above body text but well below real title-page type)
//     is an embedded diagram label ("Cooling") (issue #1698).
//
// The shape guards run after demoteHeadingRuns so that a run member already
// flattened as a list entry can't "shield" its neighbours: every member of a
// run goes to "p" regardless of its own shape, and the guards judge only
// survivors in isolation.
func finalizeHeadings(blocks []htmlBlock, docModalCharHeight float64) {
	tags := make([]string, len(blocks))
	for i, b := range blocks {
		switch {
		case b.listItem:
			tags[i] = listTag
		case b.isText:
			tags[i] = headingCandidateTag(b.medHeight, docModalCharHeight)
		}
	}

	demoteHeadingRuns(blocks, tags)

	for i, b := range blocks {
		if !b.isText || tags[i] == "p" || tags[i] == listTag {
			continue
		}
		if startsLowercase(b.text) || isLoopDiagramLabel(b.text) {
			tags[i] = "p"
			continue
		}
		// Sentence punctuation and figure references are decisive across
		// both heading sizes — no real title/section heading ends in a
		// sentence or cites a figure number. The one-word mid-band guard
		// stays h1-only: a one-word h2 (above body text but under the h1
		// threshold) is a normal small section heading ("Resilience"),
		// while a one-worder that clears the h1 threshold at mid-band size
		// is an embedded figure label ("Cooling").
		if isHeadingFalsePositive(
			b.text, b.medHeight, docModalCharHeight, tags[i] == "h1",
		) {
			tags[i] = "p"
		}
	}

	for i := range blocks {
		if !blocks[i].isText {
			continue
		}
		blocks[i].tag = tags[i]
		body := escapeXMLText(blocks[i].text)
		if (tags[i] == "p" || tags[i] == listTag) && blocks[i].inline != "" {
			body = blocks[i].inline
		}
		blocks[i].html = fmt.Sprintf("<%s>%s</%s>", tags[i], body, tags[i])
	}
}

// headingCandidateTag classifies a single paragraph by height ratio alone,
// ignoring surrounding context (that's demoteHeadingRuns's job).
func headingCandidateTag(medHeight, docModalCharHeight float64) string {
	if docModalCharHeight <= 0 {
		return "p"
	}
	switch {
	case medHeight > headingH1Ratio*docModalCharHeight:
		return "h1"
	case medHeight > headingH2Ratio*docModalCharHeight:
		return "h2"
	default:
		return "p"
	}
}

// startsLowercase reports whether text's first rune is a lowercase letter.
// Text with no leading letter (empty, or starting with punctuation/digits)
// is not considered lowercase-started.
func startsLowercase(text string) bool {
	r, _ := utf8.DecodeRuneInString(text)
	return r != utf8.RuneError && unicode.IsLower(r)
}

// loopDiagramLabelRe matches text made up of one or more single uppercase
// ASCII letters separated by single spaces ("B", "R B", "B B", …) — the
// shape of a systems-diagram loop-label callout, never of a real
// title/heading (see finalizeHeadings, issue #1698).
var loopDiagramLabelRe = regexp.MustCompile(`^[A-Z](?: [A-Z])*$`)

// isLoopDiagramLabel reports whether text (after trimming surrounding
// whitespace) matches loopDiagramLabelRe.
func isLoopDiagramLabel(text string) bool {
	return loopDiagramLabelRe.MatchString(strings.TrimSpace(text))
}

// headingH1MidBandRatio bounds the size band in which a one-word h1
// candidate is treated as an embedded figure label: real one-word headings
// ("Appendix", chapter titles) are rendered at title-page size, well above
// this ratio, while diagram labels ("Cooling") sit just above the h1
// threshold.
const headingH1MidBandRatio = 1.6

// closingQuoteChars are closing punctuation that may follow a sentence's
// final punctuation mark without changing its shape ("…like a pillar.”").
const closingQuoteChars = "”’\"')]}»·"

// trailingEllipsisRe matches a trailing typographic ellipsis, spaced dots
// ("System Traps . . .") or "…" — a title can end in one, so it doesn't
// make the text sentence-shaped.
var trailingEllipsisRe = regexp.MustCompile(`(?:\s*\.\s*){3,}$`)

// endsSentencePunctuation reports whether trimmed heading-candidate text
// ends in sentence punctuation once a trailing ellipsis and trailing
// closing quotes/brackets are stripped — the shape of a sentence, numbered
// list item, or question fragment, never of a real title/heading.
func endsSentencePunctuation(text string) bool {
	stripped := strings.TrimRight(strings.TrimSpace(text), closingQuoteChars)
	stripped = strings.TrimRight(stripped, "…")
	stripped = trailingEllipsisRe.ReplaceAllString(stripped, "")
	if stripped == "" {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(stripped)
	switch r {
	case '.', '?', ',', ';', '!':
		return true
	default:
		return false
	}
}

// figureRefRe matches a figure-number reference ("Figure 39", "Figure 1 2"),
// the marker of a caption or cross-reference fragment rather than a heading.
var figureRefRe = regexp.MustCompile(`Figure \d`)

// isHeadingFalsePositive reports whether a heading candidate's text shape
// says it isn't a heading (see finalizeHeadings). isH1 controls the
// one-word mid-band guard, which applies to h1 candidates only.
func isHeadingFalsePositive(
	text string, medHeight, docModalCharHeight float64, isH1 bool,
) bool {
	trimmed := strings.TrimSpace(text)
	if endsSentencePunctuation(trimmed) || figureRefRe.MatchString(trimmed) {
		return true
	}
	if isH1 && docModalCharHeight > 0 &&
		medHeight < headingH1MidBandRatio*docModalCharHeight &&
		len(strings.Fields(trimmed)) == 1 {
		return true
	}
	return false
}

// demoteHeadingRuns rewrites tags in place, flattening chains of
// consecutive non-"p" text blocks that look like a list (see
// finalizeHeadings): candidates chain while consecutive AND rendered at a
// similar size, and a chain of 3+ is a bibliography/citation page
// regardless of its members' text shapes (issue #1654), while a 2-member
// chain counts as a list only when one member has a list entry's text
// shape (see listShapedPair) — a chapter title page's decoration banner
// (h2-sized) beside its h1-sized title is a dissimilar pair, and a
// two-line title at one size has no entry shape; flattening either would
// erase the TOC's chapter entry (issue #1698). A non-paragraph block (e.g.
// an image) breaks a chain.
func demoteHeadingRuns(blocks []htmlBlock, tags []string) {
	chainStart := -1
	prevHeight := 0.0
	flush := func(end int) {
		if chainStart >= 0 {
			n := end - chainStart
			if n >= 3 || (n == 2 && listShapedPair(blocks, chainStart)) {
				for i := chainStart; i < end; i++ {
					tags[i] = "p"
				}
			}
		}
		chainStart = -1
	}
	for i, b := range blocks {
		if !b.isText || tags[i] == "p" || tags[i] == listTag {
			flush(i)
			continue
		}
		if chainStart < 0 {
			chainStart = i
			prevHeight = b.medHeight
			continue
		}
		if heightsSimilar(prevHeight, b.medHeight) {
			prevHeight = b.medHeight
			continue
		}
		// A size jump splits the chain: decide the chain so far, start a
		// new one here.
		flush(i)
		chainStart = i
		prevHeight = b.medHeight
	}
	if chainStart >= 0 {
		flush(len(blocks))
	}
}

// listShapedPair reports whether a two-member same-size chain counts as
// list entries: at least one member has an entry's text shape (sentence
// punctuation, a figure reference, a lowercase start, or a loop-label
// shape). Without the shape requirement, a two-line title rendered at one
// size ("Leverage Points—" / "Places to I ntervene in a System") would be
// flattened and vanish from the chapter TOC.
func listShapedPair(blocks []htmlBlock, start int) bool {
	for i := start; i < start+2; i++ {
		if endsSentencePunctuation(blocks[i].text) ||
			figureRefRe.MatchString(blocks[i].text) ||
			startsLowercase(blocks[i].text) ||
			isLoopDiagramLabel(blocks[i].text) {
			return true
		}
	}
	return false
}

// headingRunSimilarity is the maximum relative height difference two
// adjacent heading candidates may have and still count as list entries
// rendered in the same font size.
const headingRunSimilarity = 0.1

func heightsSimilar(a, b float64) bool {
	if a <= 0 || b <= 0 {
		return false
	}
	if a < b {
		a, b = b, a
	}
	return a-b <= headingRunSimilarity*b
}
