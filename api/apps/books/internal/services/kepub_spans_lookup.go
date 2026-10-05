package services

import (
	"sort"
	"strings"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// doc resolves a Kobo Source or web href to a content document: exact, then
// case-insensitive, then a unique path-suffix match either way round, since
// whether the device sends full or OPF-relative paths is unverified.
func (m *spanMap) doc(name string) *docSpans {
	if name == "" {
		return nil
	}
	for i := range m.docs {
		if m.docs[i].href == name {
			return &m.docs[i]
		}
	}
	for i := range m.docs {
		if strings.EqualFold(m.docs[i].href, name) {
			return &m.docs[i]
		}
	}
	lower := strings.ToLower(name)
	var found *docSpans
	for i := range m.docs {
		href := strings.ToLower(m.docs[i].href)
		if strings.HasSuffix(href, "/"+lower) || strings.HasSuffix(lower, "/"+href) {
			if found != nil {
				return nil
			}
			found = &m.docs[i]
		}
	}
	return found
}

// position is the neutral position of a KoboSpan bookmark: its span's start.
func (m *spanMap) position(loc models.KoboLocation) *models.ReadingPosition {
	id, ok := parseSpanID(loc.Value)
	d := m.doc(loc.Source)
	if !ok || d == nil {
		return nil
	}
	for _, s := range d.spans {
		if s.id == id {
			return &models.ReadingPosition{Href: d.href, Offset: int(s.start), Page: 0}
		}
	}
	return nil
}

// location is the span containing, or nearest before, pos (the first span
// when pos precedes them all). Of spans starting at the same offset, such as
// an image's empty span and the text after it, the first wins.
func (m *spanMap) location(pos models.ReadingPosition) *models.KoboLocation {
	d := m.doc(pos.Href)
	if pos.Href == "" || d == nil || len(d.spans) == 0 {
		return nil
	}
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
