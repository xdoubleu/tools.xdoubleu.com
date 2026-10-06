package services

import (
	"encoding/xml"
	"sort"
	"strconv"
	"strings"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// pageAnchorOf is the PDF page whose start e marks, 0 when it marks none.
func pageAnchorOf(e xml.StartElement) int {
	for _, a := range e.Attr {
		if a.Name.Local == "id" {
			return parsePageAnchorID(a.Value)
		}
	}
	return 0
}

// parsePageAnchorID reads N from a pdfpage-N id, 0 for any other id.
func parsePageAnchorID(id string) int {
	rest, ok := strings.CutPrefix(id, pageAnchorPrefix)
	if !ok {
		return 0
	}
	n, err := strconv.ParseUint(rest, 10, 31)
	if err != nil {
		return 0
	}
	return int(n)
}

// pageAt is the page whose anchor precedes offset in docs[doc]; 1 before
// every anchor (a cover), 0 when the book has none.
func (m *spanMap) pageAt(doc int, offset int32) int {
	if len(m.pages) == 0 {
		return 0
	}
	page := 1
	for _, a := range m.pages {
		if int(a.doc) > doc || (int(a.doc) == doc && a.start > offset) {
			break
		}
		page = int(a.page)
	}
	return page
}

// anchorFor is the anchor of page, else of the nearest page before it that
// has one (a blank page has none), else the first.
func (m *spanMap) anchorFor(page int) (pageAnchor, bool) {
	best := -1
	for i, a := range m.pages {
		if int(a.page) <= page && (best < 0 || a.page > m.pages[best].page) {
			best = i
		}
	}
	switch {
	case best >= 0:
		return m.pages[best], true
	case len(m.pages) > 0:
		return m.pages[0], true
	default:
		return pageAnchor{page: 0, doc: 0, start: 0}, false
	}
}

// pageLocation is the first span at or after page's anchor.
func (m *spanMap) pageLocation(page int) *models.KoboLocation {
	a, ok := m.anchorFor(page)
	if !ok {
		return nil
	}
	for doc := int(a.doc); doc < len(m.docs); doc++ {
		spans := m.docs[doc].spans
		i := 0
		if doc == int(a.doc) {
			i = sort.Search(len(spans), func(i int) bool { return spans[i].start >= a.start })
		}
		if i < len(spans) {
			return &models.KoboLocation{
				Source: m.docs[doc].href, Type: koboSpanType, Value: spans[i].id.String(),
			}
		}
	}
	return nil
}

// readerPosition is pos of a PDF-sourced book in both of the web reader's
// forms: a page gains its anchor's href and offset, an href and offset the
// page whose anchor precedes it. nil when pos can't be translated.
func (m *spanMap) readerPosition(pos models.ReadingPosition) *models.ReadingPosition {
	if !m.pdf {
		return nil
	}
	if pos.Page > 0 {
		a, ok := m.anchorFor(pos.Page)
		if !ok {
			return nil
		}
		return &models.ReadingPosition{
			Href: m.docs[a.doc].href, Offset: int(a.start), Page: pos.Page,
		}
	}
	doc := m.docIndex(pos.Href)
	if doc < 0 {
		return nil
	}
	//nolint:gosec // offsets are validated non-negative and under 2^31
	page := m.pageAt(doc, int32(pos.Offset))
	if page == 0 {
		return nil
	}
	return &models.ReadingPosition{Href: m.docs[doc].href, Offset: pos.Offset, Page: page}
}
