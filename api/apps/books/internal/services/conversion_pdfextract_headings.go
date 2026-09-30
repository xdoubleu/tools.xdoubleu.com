package services

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// finalizeHeadings tags each paragraph block h1/h2/p by its height ratio
// to the document's modal body height, then demotes a candidate to "p" when
// it runs as a list (demoteHeadingRuns, first, so list members can't shield
// each other) or has a non-heading shape: a lowercase start, loop-diagram
// letters ("R B"), sentence punctuation, a figure reference, or a single
// mid-band word (a diagram label). It renders each block's html in place.
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
		// The one-word mid-band guard is h1-only: a one-word h2 is a normal
		// section heading ("Resilience"), a one-word h1 at mid-band size an
		// embedded figure label ("Cooling").
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

// demoteHeadingRuns flattens chains of consecutive heading candidates at a
// similar size to "p": a chain of 3+ is a bibliography or citation page, a
// pair only when a member has a list entry's shape (listShapedPair), so a
// chapter banner beside its title, or a two-line title, keeps its heading.
// A non-paragraph block breaks a chain.
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
