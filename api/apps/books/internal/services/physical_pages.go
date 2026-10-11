package services

import (
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"

	"tools.xdoubleu.com/apps/books/internal/models"
)

const (
	// maxExcerptRunes caps the "continue on page" sentence.
	maxExcerptRunes = 200
	// minSentenceRunes skips a sentence end too early to be one ("Mr.").
	minSentenceRunes = 20
)

// pagePoint pins a physical page's start to a global offset.
type pagePoint struct{ page, at int }

func (t *bookText) total() int { return t.cum[len(t.docs)] }

func (t *bookText) docIndex(href string) int {
	for i, d := range t.docs {
		if d.href == href {
			return i
		}
	}
	return -1
}

// global is pos's offset across the spine; false when pos isn't in it.
func (t *bookText) global(pos models.ReadingPosition) (int, bool) {
	if pos.Page > 0 {
		return 0, false
	}
	i := t.docIndex(pos.Href)
	if i < 0 {
		return 0, false
	}
	return t.cum[i] + min(max(pos.Offset, 0), len(t.docs[i].units)), true
}

// position is the neutral position at global offset g.
func (t *bookText) position(g int) models.ReadingPosition {
	i := sort.Search(len(t.docs), func(i int) bool { return t.cum[i+1] > g })
	if i == len(t.docs) {
		i = len(t.docs) - 1
	}
	return models.ReadingPosition{
		Href: t.docs[i].href, Offset: min(g-t.cum[i], len(t.docs[i].units)), Page: 0,
	}
}

// percentAt is g's share of the text, as the reading state's percent.
func (t *bookText) percentAt(g int) int {
	if t.total() == 0 {
		return 0
	}
	return min(g*models.MaxProgressPercent/t.total(), models.MaxProgressPercent)
}

// hasPrintPages reports whether the EPUB carries the print edition's page
// numbers, which then map pages exactly.
func (t *bookText) hasPrintPages() bool {
	return len(t.arabicPages()) > 0
}

// arabicPages are the print pages with a numeric label; roman front matter
// has no page the user can enter.
func (t *bookText) arabicPages() []pagePoint {
	var pts []pagePoint
	for _, p := range t.printPages {
		if n, err := strconv.Atoi(p.label); err == nil && n > 0 {
			pts = append(pts, pagePoint{page: n, at: p.at})
		}
	}
	return pts
}

// anchorPoints are c's pages pinned to offsets, by page and offset both
// increasing: the text start, the user's anchors, and the end of the text
// after the last page. A user anchor overrides a default it conflicts with,
// by page or by offset; any other anchor out of order is dropped.
func (t *bookText) anchorPoints(c models.PhysicalCopy) []pagePoint {
	if c.PageCount <= 0 {
		return nil
	}
	first := max(c.TextStartPage, 1)
	var user []pagePoint
	for _, a := range c.Anchors {
		g, ok := t.global(models.ReadingPosition{Href: a.Href, Offset: a.Offset, Page: 0})
		if ok && a.Page > 0 {
			user = append(user, pagePoint{page: a.Page, at: g})
		}
	}
	sort.SliceStable(user, func(i, j int) bool { return user[i].page < user[j].page })

	pts := []pagePoint{}
	if len(user) == 0 || user[0].page > first && user[0].at > t.start {
		pts = append(pts, pagePoint{page: first, at: t.start})
	}
	for _, p := range user {
		if n := len(pts); n == 0 || p.page > pts[n-1].page && p.at > pts[n-1].at {
			pts = append(pts, p)
		}
	}
	end := pagePoint{page: c.PageCount + 1, at: t.total()}
	if n := len(pts); n > 0 && end.page > pts[n-1].page && end.at > pts[n-1].at {
		pts = append(pts, end)
	}
	if len(pts) < 2 { //nolint:mnd // interpolation needs a segment
		return nil
	}
	return pts
}

// positionOfPage is where page starts in the EPUB; false when c can't be
// mapped.
func (t *bookText) positionOfPage(
	c models.PhysicalCopy, page int,
) (models.ReadingPosition, int, bool) {
	var g int
	if pts := t.arabicPages(); len(pts) > 0 {
		g = pts[0].at
		for _, p := range pts {
			if p.page == page {
				g = p.at
				break
			}
			if p.page < page {
				g = p.at
			}
		}
	} else {
		pts = t.anchorPoints(c)
		if pts == nil {
			return models.ReadingPosition{Href: "", Offset: 0, Page: 0}, 0, false
		}
		g = interpolate(pts, page, func(p pagePoint) int { return p.page },
			func(p pagePoint) int { return p.at }, true)
	}
	return t.position(g), g, true
}

// pageOf is the physical page pos is on, 0 when it can't be mapped.
func (t *bookText) pageOf(c models.PhysicalCopy, pos models.ReadingPosition) int {
	g, ok := t.global(pos)
	if !ok {
		return 0
	}
	if pts := t.arabicPages(); len(pts) > 0 {
		page := pts[0].page
		for _, p := range pts {
			if p.at > g {
				break
			}
			page = p.page
		}
		return page
	}
	pts := t.anchorPoints(c)
	if pts == nil {
		return 0
	}
	return interpolate(pts, g, func(p pagePoint) int { return p.at },
		func(p pagePoint) int { return p.page }, false)
}

// interpolate maps x along pts' piecewise-linear from → to, clamping to the
// first point and to just before the last. A page maps to the first offset on
// it (up), an offset to the page it's on (down), so a page round-trips.
func interpolate(pts []pagePoint, x int, from, to func(pagePoint) int, up bool) int {
	last := len(pts) - 1
	switch {
	case x <= from(pts[0]):
		return to(pts[0])
	case x >= from(pts[last]):
		return max(to(pts[last])-1, to(pts[0]))
	}
	i := sort.Search(last, func(i int) bool { return from(pts[i+1]) > x })
	a, b := pts[i], pts[i+1]
	num, den := (x-from(a))*(to(b)-to(a)), from(b)-from(a)
	if up {
		num += den - 1
	}
	return to(a) + num/den
}

// excerpt is the first sentence from pos, to find on the physical page.
func (t *bookText) excerpt(pos models.ReadingPosition) string {
	i := t.docIndex(pos.Href)
	if i < 0 || pos.Page > 0 {
		return ""
	}
	d := t.docs[i]
	from := min(max(pos.Offset, 0), len(d.units))
	to := min(len(d.units), from+4*maxExcerptRunes)
	spaced := make([]uint16, 0, to-from+len(d.gaps))
	g := sort.SearchInts(d.gaps, from+1)
	for at := from; at < to; at++ {
		for g < len(d.gaps) && d.gaps[g] == at {
			spaced = append(spaced, ' ')
			g++
		}
		spaced = append(spaced, d.units[at])
	}
	text := []rune(collapseSpace(string(utf16.Decode(spaced))))
	for n, r := range text {
		if n+1 >= minSentenceRunes && strings.ContainsRune(".!?…", r) &&
			(n+1 == len(text) || unicode.IsSpace(text[n+1])) {
			text = text[:n+1]
			break
		}
	}
	if len(text) <= maxExcerptRunes {
		return string(text)
	}
	cut := string(text[:maxExcerptRunes])
	if sp := strings.LastIndexByte(cut, ' '); sp > 0 {
		cut = cut[:sp]
	}
	return cut + "…"
}
