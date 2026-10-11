//nolint:testpackage // testing the unexported page mapping
package services

import (
	"context"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/mocks"
	"tools.xdoubleu.com/apps/books/internal/models"
)

func copyOf(
	pageCount, start int,
	anchors ...models.PhysicalAnchor,
) models.PhysicalCopy {
	return models.PhysicalCopy{
		PageCount: pageCount, TextStartPage: start, Anchors: anchors, Syncable: true,
	}
}

func anchorAt(page int, href string, offset int) models.PhysicalAnchor {
	return models.PhysicalAnchor{Page: page, Href: href, Offset: offset}
}

func TestBookText_ArabicPagesSkipZeroAndOutOfRange(t *testing.T) {
	bt := &bookText{
		docs: nil, cum: []int{0}, start: 0, chapters: nil,
		printPages: []printPage{
			{label: "0", at: 0},
			{label: "x", at: 1},
			{label: "1", at: 5},
			{label: "99999999999", at: 9},
			{label: "-3", at: 9},
		},
	}
	assert.Equal(t, []pagePoint{{page: 1, at: 5}}, bt.arabicPages())
}

func TestBookText_PrintPageWithoutMarkerUsesTheOneBefore(t *testing.T) {
	bt := &bookText{
		docs: []textDoc{{href: "a", units: make([]uint16, 100), gaps: nil}},
		cum:  []int{0, 100}, start: 0, chapters: nil,
		printPages: []printPage{
			{label: "1", at: 0},
			{label: "3", at: 40},
			{label: "4", at: 60},
		},
	}
	c := copyOf(0, 0)
	for page, want := range map[int]int{1: 0, 2: 0, 3: 40, 4: 60, 9: 60} {
		got, _, ok := bt.positionOfPage(c, page)
		require.True(t, ok)
		assert.Equal(t, want, got.Offset, "page %d", page)
	}
}

func TestBookText_AnchorEdgeCases(t *testing.T) {
	bt := physicalText(t, false)
	ch2 := bt.position(bt.chapters[1].at)
	ch3 := pos(mocks.PhysicalCh3Href, 0)
	end := pos(mocks.PhysicalCh3Href, 1<<20)

	_, _, ok := bt.positionOfPage(copyOf(0, 1, anchorAt(5, ch2.Href, ch2.Offset)), 5)
	assert.False(t, ok, "no page count, even with an anchor")

	c := copyOf(99, 5, anchorAt(0, ch2.Href, ch2.Offset))
	assert.Equal(
		t,
		5,
		bt.pageOf(c, pos(mocks.PhysicalCh1Href, 0)),
		"a page-0 anchor is ignored",
	)

	c = copyOf(99, 5, anchorAt(5, ch2.Href, ch2.Offset))
	assert.Equal(t, 5, bt.pageOf(c, ch2), "an anchor on the text-start page replaces it")

	c = copyOf(99, 1, anchorAt(9, mocks.PhysicalCh1Href, 0))
	assert.Equal(t, 9, bt.pageOf(c, pos(mocks.PhysicalCh1Href, 0)),
		"an anchor at the text start replaces it")

	c = copyOf(99, 1, anchorAt(30, ch2.Href, ch2.Offset), anchorAt(30, ch3.Href, 0))
	assert.Equal(t, 30, bt.pageOf(c, ch2), "the first of two anchors on a page wins")
	got, _, _ := bt.positionOfPage(c, 30)
	assert.Equal(t, ch2, got)

	c = copyOf(
		99,
		1,
		anchorAt(30, ch2.Href, ch2.Offset),
		anchorAt(40, ch2.Href, ch2.Offset),
	)
	assert.Equal(t, 30, bt.pageOf(c, ch2), "a later page at the same offset is dropped")
	got, _, _ = bt.positionOfPage(c, 35)
	assert.Equal(t, mocks.PhysicalCh2Href, got.Href)

	c = copyOf(99, 1, anchorAt(50, end.Href, end.Offset))
	assert.Equal(t, 49, bt.pageOf(c, end), "an anchor at the end replaces it")
}

func textOf(s string) *bookText {
	units := utf16.Encode([]rune(s))
	return &bookText{
		docs: []textDoc{{href: "a", units: units, gaps: nil}},
		cum:  []int{0, len(units)}, start: 0, printPages: nil, chapters: nil,
	}
}

func TestBookText_ExcerptBoundaries(t *testing.T) {
	twenty := "Abcdefghijklmnopqrs." // a sentence of exactly minSentenceRunes
	assert.Equal(t, twenty, textOf(twenty+" Next one here.").excerpt(pos("a", 0)))

	exact := strings.Repeat("a", maxExcerptRunes)
	assert.Equal(t, exact, textOf(exact).excerpt(pos("a", 0)), "fits without a cut")

	unbroken := strings.Repeat("b", maxExcerptRunes+5)
	assert.Equal(t, strings.Repeat("b", maxExcerptRunes)+"…",
		textOf(unbroken).excerpt(pos("a", 0)), "no space to cut at")
}

func TestBookText_ExcerptSpacesBlocks(t *testing.T) {
	bt := physicalText(t, false)
	assert.Equal(t, "pigraph. Two Chapter 2 sentence 01 is right here.",
		bt.excerpt(pos(mocks.PhysicalCh2Href, 1)))
}

func TestScanBookDoc_GapsAndMarkers(t *testing.T) {
	s, err := scanBookDoc(strings.NewReader(`<html><body><p>A</p><p>B</p>`+
		`<span epub:type="pagebreak" id="123"/><span role="doc-pagebreak" id="p19"/>`+
		`</body></html>`), nil)
	require.NoError(t, err)
	assert.Equal(t, []int{0, 1, 2}, s.gaps, "one gap per boundary")
	assert.Equal(t, []printPage{{label: "123", at: 2}, {label: "19", at: 2}}, s.breaks)
}

func TestScanZipDoc_SizeLimit(t *testing.T) {
	doc := func(size int) string {
		head, tail := "<html><body><p>", "</p></body></html>"
		return head + strings.Repeat("a", size-len(head)-len(tail)) + tail
	}
	zr := zipOf(t, map[string]string{
		"max.xhtml":  doc(maxContentDocBytes),
		"over.xhtml": doc(maxContentDocBytes + 1),
	})
	assert.NotEmpty(t, scanZipDoc(zr, "max.xhtml", nil).units)
	assert.Empty(t, scanZipDoc(zr, "over.xhtml", nil).units)
	_, err := buildBookText(context.Background(), zr)
	require.Error(t, err)
}
