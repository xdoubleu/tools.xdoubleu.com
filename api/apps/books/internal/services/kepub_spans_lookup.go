package services

import (
	"sort"
	"strings"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// docIndex resolves a Kobo Source or web href to a content document (-1 for
// none): exact, then case-insensitive, then a unique path-suffix match either
// way round, since whether the device sends full or OPF-relative paths is
// unverified.
func (m *spanMap) docIndex(name string) int {
	if name == "" {
		return -1
	}
	for i := range m.docs {
		if m.docs[i].href == name {
			return i
		}
	}
	for i := range m.docs {
		if strings.EqualFold(m.docs[i].href, name) {
			return i
		}
	}
	lower := strings.ToLower(name)
	found := -1
	for i := range m.docs {
		href := strings.ToLower(m.docs[i].href)
		if strings.HasSuffix(href, "/"+lower) || strings.HasSuffix(lower, "/"+href) {
			if found >= 0 {
				return -1
			}
			found = i
		}
	}
	return found
}

// position is the neutral position of a KoboSpan bookmark: its span's start,
// or for a PDF-sourced KEPUB the page whose anchor precedes it.
func (m *spanMap) position(loc models.KoboLocation) *models.ReadingPosition {
	id, ok := parseSpanID(loc.Value)
	doc := m.docIndex(loc.Source)
	if !ok || doc < 0 {
		return nil
	}
	d := &m.docs[doc]
	for _, s := range d.spans {
		if s.id != id {
			continue
		}
		if !m.pdf {
			return &models.ReadingPosition{Href: d.href, Offset: int(s.start), Page: 0}
		}
		if page := m.pageAt(doc, s.start); page > 0 {
			return &models.ReadingPosition{Href: "", Offset: 0, Page: page}
		}
		return nil
	}
	return nil
}

// location is the span containing, or nearest before, pos (the first span
// when pos precedes them all). Of spans starting at the same offset, such as
// an image's empty span and the text after it, the first wins. A page maps
// to the first span at or after its anchor.
func (m *spanMap) location(pos models.ReadingPosition) *models.KoboLocation {
	if pos.Page > 0 {
		if !m.pdf {
			return nil
		}
		return m.pageLocation(pos.Page)
	}
	doc := m.docIndex(pos.Href)
	if doc < 0 || len(m.docs[doc].spans) == 0 {
		return nil
	}
	d := &m.docs[doc]
	i := sort.Search(len(d.spans), func(i int) bool {
		return int(d.spans[i].start) > pos.Offset
	}) - 1
	i = max(i, 0)
	for i > 0 && d.spans[i-1].start == d.spans[i].start {
		i--
	}
	return &models.KoboLocation{
		Source: d.href, Type: koboSpanType, Value: d.spans[i].id.String(),
	}
}
