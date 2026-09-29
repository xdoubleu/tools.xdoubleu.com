package services

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// listTag is the htmlBlock tag of a bulleted list item.
const listTag = "li"

var (
	// listMarkerRe matches a line opening with a list marker: a bullet, a
	// number ("12." / "3)"), or a letter item ("B)"), then whitespace.
	listMarkerRe = regexp.MustCompile(`^(?:[•◦▪‣–]|\d{1,3}[.)]|[A-Za-z]\))\s`)
	// bulletMarkerRe matches a bulleted item's marker. Only bullets become
	// <li>: numbered items keep their literal number, which a reader would
	// otherwise repeat with its own list numbering.
	bulletMarkerRe = regexp.MustCompile(`^[•◦▪‣–]\s+`)
)

// markerTextStart returns the x where a marker line's text begins after its
// marker — where a hanging continuation line aligns.
func markerTextStart(l pdfLine) (float64, bool) {
	marker := listMarkerRe.FindString(l.text)
	if marker == "" {
		return 0, false
	}
	n := utf8.RuneCountInString(strings.TrimSpace(marker))
	if len(l.chars) <= n {
		return 0, false
	}
	return l.chars[n].left, true
}

// asListItem turns a paragraph opening with a bullet into a list item with
// the bullet stripped.
func asListItem(b htmlBlock) htmlBlock {
	marker := bulletMarkerRe.FindString(b.text)
	if marker == "" {
		return b
	}
	b.text = strings.TrimPrefix(b.text, marker)
	b.listItem = true
	return b
}

// writeBlocksHTML writes each block's html, wrapping each run of list items
// in one <ul>.
func writeBlocksHTML(b *strings.Builder, blocks []htmlBlock) {
	inList := false
	for _, blk := range blocks {
		isItem := blk.tag == listTag
		if isItem != inList {
			if isItem {
				b.WriteString("<ul>\n")
			} else {
				b.WriteString("</ul>\n")
			}
			inList = isItem
		}
		b.WriteString(blk.html)
		b.WriteByte('\n')
	}
	if inList {
		b.WriteString("</ul>\n")
	}
}
