package services

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// A print-shop proof slug (issue #1652) is running production metadata a
// pre-press vendor stamps on every page of a proof PDF — a page-number
// token, a typesetting date, and a time, e.g. "TIS final pgs 72 5/2/09
// 10:37:39". Left alone it survives paragraph extraction as its own
// spurious body paragraph, once per page. It has a recognizable shape (a
// short line combining all three tokens) and a recognizable position
// (recurring at the very top or bottom of many pages' block list) — this
// file detects that shape+position combination and drops the matching
// blocks, without hardcoding any book-specific literal text.
const (
	// proofSlugMaxTokens/proofSlugMaxLineLength bound how "short" a line
	// must be to even be considered — a genuine body paragraph that happens
	// to mention a date is normally much longer than this.
	proofSlugMaxTokens     = 12
	proofSlugMaxLineLength = 80
	// minProofSlugOccurrences: the shape must recur at the same
	// page-boundary position on at least this many pages before it's
	// treated as running metadata rather than a one-off coincidental match
	// in real body text.
	minProofSlugOccurrences = 3
)

var (
	proofSlugDateRe    = regexp.MustCompile(`^\d{1,2}/\d{1,2}/\d{2,4}$`)
	proofSlugTimeRe    = regexp.MustCompile(`^\d{1,2}:\d{2}:\d{2}$`)
	proofSlugPageNumRe = regexp.MustCompile(`^\d{1,4}$`)
)

// isProofSlugLine reports whether text has the shape of a print-shop proof
// slug: a short line whose whitespace-delimited tokens include a bare
// page-number token, an m/d/yy-style date token, and an h:mm:ss-style time
// token, each as its own token (so a paragraph that merely mentions a date
// in running prose, without a standalone page number and time alongside it,
// never matches).
func isProofSlugLine(text string) bool {
	if utf8.RuneCountInString(text) > proofSlugMaxLineLength {
		return false
	}
	tokens := strings.Fields(text)
	if len(tokens) == 0 || len(tokens) > proofSlugMaxTokens {
		return false
	}

	var hasPageNum, hasDate, hasTime bool
	for _, tok := range tokens {
		switch {
		case proofSlugDateRe.MatchString(tok):
			hasDate = true
		case proofSlugTimeRe.MatchString(tok):
			hasTime = true
		case proofSlugPageNumRe.MatchString(tok):
			hasPageNum = true
		}
	}
	return hasPageNum && hasDate && hasTime
}

// removeProofSlugLines drops per-page proof-slug blocks from a
// document already split into per-page block lists. Only the very first or
// very last block of a page is ever a candidate (proof slugs are printed at
// a page's top or bottom margin), and a candidate position is only acted on
// once it recurs on at least minProofSlugOccurrences pages — ruling out
// stripping a one-off paragraph that coincidentally matches the shape.
func removeProofSlugLines(pages [][]htmlBlock) [][]htmlBlock {
	isTop := make([]bool, len(pages))
	isBottom := make([]bool, len(pages))
	var topCount, bottomCount int

	for i, blocks := range pages {
		if len(blocks) == 0 {
			continue
		}
		if first := blocks[0]; first.tag == "p" && isProofSlugLine(first.text) {
			isTop[i] = true
			topCount++
		}
		last := blocks[len(blocks)-1]
		sameAsFirst := len(blocks) == 1
		if last.tag == "p" && isProofSlugLine(last.text) && !sameAsFirst {
			isBottom[i] = true
			bottomCount++
		}
	}

	dropTop := topCount >= minProofSlugOccurrences
	dropBottom := bottomCount >= minProofSlugOccurrences
	if !dropTop && !dropBottom {
		return pages
	}

	result := make([][]htmlBlock, len(pages))
	for i, blocks := range pages {
		filtered := blocks
		if dropBottom && isBottom[i] {
			filtered = filtered[:len(filtered)-1]
		}
		if dropTop && isTop[i] && len(filtered) > 0 {
			filtered = filtered[1:]
		}
		result[i] = filtered
	}
	return result
}
