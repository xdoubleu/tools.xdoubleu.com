package services

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// A print-shop proof slug ("TIS final pgs 72 5/2/09 10:37:39") is vendor
// metadata stamped on every page. It is detected by shape (page number, date
// and time tokens on a short line) plus recurring top/bottom position, never
// by literal text.
const (
	// Genuine paragraphs mentioning a date are normally longer than this.
	proofSlugMaxTokens     = 12
	proofSlugMaxLineLength = 80
	// The shape must recur on this many pages to rule out coincidence.
	minProofSlugOccurrences = 3
)

var (
	proofSlugDateRe    = regexp.MustCompile(`^\d{1,2}/\d{1,2}/\d{2,4}$`)
	proofSlugTimeRe    = regexp.MustCompile(`^\d{1,2}:\d{2}:\d{2}$`)
	proofSlugPageNumRe = regexp.MustCompile(`^\d{1,4}$`)
)

// isProofSlugLine reports a short line with standalone page-number, m/d/yy
// date and h:mm:ss time tokens.
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

// removeProofSlugLines drops a page's first or last block when it has the slug
// shape and that position matches on at least minProofSlugOccurrences pages.
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
