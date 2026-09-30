package services

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// minRunningHeaderPages: a folioed page-edge line whose text, page
	// number aside, recurs at a page edge on this many pages is a running
	// header or footer.
	minRunningHeaderPages = 2
	// runningHeaderMaxLength bounds how long a running header may be.
	runningHeaderMaxLength = 80
	// continuationHeightRatio: a paragraph continued on the next page keeps
	// its size within this relative difference.
	continuationHeightRatio = 0.3
	// footnoteHeightRatio: text set below this share of the body text size
	// at the foot of a page is a footnote.
	footnoteHeightRatio = 0.9
)

var (
	// folioRe matches a page-number token at either end of a header line:
	// arabic, or roman as front matter numbers its pages.
	folioRe = regexp.MustCompile(
		`^(?:\d{1,4}|[ivxlcdm]{1,7}|[IVXLCDM]{1,7})\s+|` +
			`\s+(?:\d{1,4}|[ivxlcdm]{1,7}|[IVXLCDM]{1,7})$`,
	)
	bareFolioRe = regexp.MustCompile(`^(?:\d{1,4}|[ivxlcdm]{1,7}|[IVXLCDM]{1,7})$`)
)

// runningHeaderKey returns a page-edge block's text with its page number
// removed, and whether it carried one.
func runningHeaderKey(b htmlBlock) (string, bool) {
	text := strings.TrimSpace(b.text)
	if bareFolioRe.MatchString(text) {
		return "", true
	}
	key := folioRe.ReplaceAllString(text, "")
	return key, key != text
}

// pageEdges returns the indexes of a page's first and last blocks.
func pageEdges(blocks []htmlBlock) []int {
	switch len(blocks) {
	case 0:
		return nil
	case 1:
		return []int{0}
	default:
		return []int{0, len(blocks) - 1}
	}
}

// removeRunningHeaders drops a page's first or last block below heading size
// that carries a page number and whose text without it recurs at a page
// edge on minRunningHeaderPages pages — a short section's title opening the
// page before counts. Unfolioed lines can't be told from recurring headings.
func removeRunningHeaders(pages [][]htmlBlock, docModalHeight float64) [][]htmlBlock {
	counts := edgeKeyCounts(pages)
	result := make([][]htmlBlock, len(pages))
	for p, blocks := range pages {
		drop := map[int]bool{}
		for _, i := range pageEdges(blocks) {
			b := blocks[i]
			key, folioed := runningHeaderKey(b)
			if b.isText && folioed && counts[key] >= minRunningHeaderPages &&
				(docModalHeight <= 0 || b.medHeight < headingH1Ratio*docModalHeight) {
				drop[i] = true
			}
		}
		kept := make([]htmlBlock, 0, len(blocks))
		for i, b := range blocks {
			if !drop[i] {
				kept = append(kept, b)
			}
		}
		result[p] = kept
	}
	return result
}

// edgeKeyCounts counts, per running-header key, the pages with a short
// text block at an edge carrying it.
func edgeKeyCounts(pages [][]htmlBlock) map[string]int {
	counts := map[string]int{}
	for _, blocks := range pages {
		seen := map[string]bool{}
		for _, i := range pageEdges(blocks) {
			if !blocks[i].isText ||
				utf8.RuneCountInString(blocks[i].text) > runningHeaderMaxLength {
				continue
			}
			key, _ := runningHeaderKey(blocks[i])
			if !seen[key] {
				seen[key] = true
				counts[key]++
			}
		}
	}
	return counts
}

// joinPageContinuations rejoins paragraphs split by a page break: a page's
// last body paragraph that doesn't end a sentence absorbs the next page's
// first text paragraph when that starts in lowercase at a similar size.
// Figures, notes, and footnotes (text set smaller than body text) after the
// first half stay after the joined paragraph.
func joinPageContinuations(pages [][]htmlBlock, docModalHeight float64) [][]htmlBlock {
	var last *htmlBlock
	for p := range pages {
		blocks := pages[p]
		if j := firstTextBlock(blocks); last != nil && j >= 0 &&
			continuesOnNextPage(*last, blocks[j]) {
			next := blocks[j]
			last.inline = joinContinuationHTML(
				last.text,
				last.inline,
				next.text,
				next.inline,
			)
			last.text = joinContinuation(last.text, next.text)
			pages[p] = append(blocks[:j:j], blocks[j+1:]...)
		}
		for i := len(pages[p]) - 1; i >= 0; i-- {
			if b := &pages[p][i]; b.isText &&
				b.medHeight >= footnoteHeightRatio*docModalHeight {
				last = b
				break
			}
		}
	}
	return pages
}

func firstTextBlock(blocks []htmlBlock) int {
	for i, b := range blocks {
		if b.isText {
			return i
		}
	}
	return -1
}

func continuesOnNextPage(prev, next htmlBlock) bool {
	if endsSentence(prev.text) || !startsLowercase(next.text) {
		return false
	}
	lo, hi := min(prev.medHeight, next.medHeight), max(prev.medHeight, next.medHeight)
	return lo > 0 && hi-lo <= continuationHeightRatio*lo
}

// endsSentence reports whether text ends a sentence or introduces what
// follows, closing quotes and brackets aside.
func endsSentence(text string) bool {
	trimmed := strings.TrimRight(strings.TrimSpace(text), closingQuoteChars)
	r, _ := utf8.DecodeLastRuneInString(trimmed)
	return strings.ContainsRune(".!?:…", r)
}

// joinContinuation joins two halves of a paragraph, closing a word
// hyphenated across the break.
func joinContinuation(a, b string) string {
	if strings.HasSuffix(a, "-") && startsLower(b) {
		return strings.TrimSuffix(a, "-") + b
	}
	return a + " " + b
}
