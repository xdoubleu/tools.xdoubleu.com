//nolint:testpackage // testing unexported service helpers
package services

import (
	"strconv"
	"testing"

	"github.com/klippa-app/go-pdfium/enums"
	"github.com/stretchr/testify/require"
)

func TestSplitAuthors(t *testing.T) {
	t.Parallel()
	require.Nil(t, splitAuthors(""))
	require.Equal(t, []string{"Ada Lovelace"}, splitAuthors("Ada Lovelace"))
	require.Equal(
		t,
		[]string{"Ada Lovelace", "Alan Turing"},
		splitAuthors("Ada Lovelace, Alan Turing"),
	)
	require.Equal(
		t,
		[]string{"Ada Lovelace", "Alan Turing"},
		splitAuthors("Ada Lovelace; Alan Turing;"),
	)
	require.Equal(t, []string{"Solo Author"}, splitAuthors("  Solo Author  "))
}

func TestClampBin(t *testing.T) {
	t.Parallel()
	require.Equal(t, 0, clampBin(-5, 10))
	require.Equal(t, 0, clampBin(0, 10))
	require.Equal(t, 5, clampBin(5, 10))
	require.Equal(t, 9, clampBin(9, 10))
	require.Equal(t, 9, clampBin(10, 10))
	require.Equal(t, 9, clampBin(100, 10))
}

func TestPixelAt(t *testing.T) {
	t.Parallel()

	buf := []byte{10, 20, 30, 40, 50, 60, 70, 80}

	r, g, b, a := pixelAt(buf, 0, 0, enums.FPDF_BITMAP_FORMAT_BGRA)
	require.Equal(t, []byte{r, g, b, a}, []byte{30, 20, 10, 40})

	r, g, b, a = pixelAt(buf, 0, 0, enums.FPDF_BITMAP_FORMAT_BGRX)
	require.Equal(t, []byte{r, g, b, a}, []byte{30, 20, 10, 255})

	r, g, b, a = pixelAt(buf[:6], 0, 0, enums.FPDF_BITMAP_FORMAT_BGR)
	require.Equal(t, []byte{r, g, b, a}, []byte{30, 20, 10, 255})

	r, g, b, a = pixelAt(buf, 0, 3, enums.FPDF_BITMAP_FORMAT_GRAY)
	require.Equal(t, []byte{r, g, b, a}, []byte{40, 40, 40, 255})

	// Out-of-bounds reads return transparent black instead of panicking.
	r, g, b, a = pixelAt(buf, 0, 100, enums.FPDF_BITMAP_FORMAT_BGRA)
	require.Equal(t, []byte{r, g, b, a}, []byte{0, 0, 0, 0})
	r, g, b, a = pixelAt(buf, 0, 100, enums.FPDF_BITMAP_FORMAT_BGRX)
	require.Equal(t, []byte{r, g, b, a}, []byte{0, 0, 0, 0})
	r, g, b, a = pixelAt(buf, 0, 100, enums.FPDF_BITMAP_FORMAT_BGR)
	require.Equal(t, []byte{r, g, b, a}, []byte{0, 0, 0, 0})
	r, g, b, a = pixelAt(buf, 0, 100, enums.FPDF_BITMAP_FORMAT_GRAY)
	require.Equal(t, []byte{r, g, b, a}, []byte{0, 0, 0, 0})

	r, g, b, a = pixelAt(buf, 0, 0, enums.FPDF_BITMAP_FORMAT_UNKNOWN)
	require.Equal(t, []byte{r, g, b, a}, []byte{0, 0, 0, 255})
}

func TestMedianEmpty(t *testing.T) {
	t.Parallel()
	require.InDelta(t, 0, median(nil), 0.0001)
}

func TestJoinLinesWithHyphenation_Empty(t *testing.T) {
	t.Parallel()
	require.Equal(t, "", joinLinesWithHyphenation(nil))
}

func TestFigureTracker_CapAndDedupe(t *testing.T) {
	t.Parallel()
	tracker := newFigureTracker()

	name, ok := tracker.accept([]byte("a"))
	require.True(t, ok)
	require.Equal(t, "fig-0.png", name)

	// Same bytes again: dedupe rejects it.
	_, ok = tracker.accept([]byte("a"))
	require.False(t, ok)

	name, ok = tracker.accept([]byte("b"))
	require.True(t, ok)
	require.Equal(t, "fig-1.png", name)

	tracker.count = figureMaxPerDoc
	_, ok = tracker.accept([]byte("c"))
	require.False(t, ok)
}

func TestFindGutter_DegenerateInputs(t *testing.T) {
	t.Parallel()
	l, r, ok := findGutter(nil, 100, 100)
	require.False(t, ok)
	require.Zero(t, l)
	require.Zero(t, r)

	oneChar := []pdfChar{ //nolint:exhaustruct // text unused by findGutter
		{left: 0, right: 10, top: 10, bottom: 0},
	}
	_, _, ok = findGutter(oneChar, 0, 100)
	require.False(t, ok)
}

func TestIsProofSlugLine(t *testing.T) {
	t.Parallel()

	// True positives: page number + date + time tokens, in any order,
	// within a short line.
	require.True(t, isProofSlugLine("TIS final pgs 72 5/2/09 10:37:39"))
	require.True(t, isProofSlugLine("72 5/2/09 10:37:39"))
	require.True(t, isProofSlugLine("10:37:39 5/2/09 72"))

	// A genuine body paragraph mentioning a date, but missing the other two
	// tokens, must never match.
	require.False(t, isProofSlugLine(
		"The revised schedule set the deadline for 5/2/09 according to the "+
			"committee notes",
	))
	// A short line with only a page number, no date/time.
	require.False(t, isProofSlugLine("72"))
	// A short line with a date and time but no bare page-number token.
	require.False(t, isProofSlugLine("Filed on 5/2/09 at 10:37:39"))
	// Too many tokens even though all three pieces are present.
	require.False(t, isProofSlugLine(
		"This much longer paragraph happens to mention page 72 and the date "+
			"5/2/09 and the time 10:37:39 in passing",
	))
	require.False(t, isProofSlugLine(""))
}

func TestRemoveProofSlugLines(t *testing.T) {
	t.Parallel()

	bodyP := func(text string) htmlBlock {
		return htmlBlock{ //nolint:exhaustruct // medHeight/isText unused by this test
			tag: "p", text: text, html: "",
		}
	}
	footer := func(page int) htmlBlock {
		return bodyP(
			"TIS final pgs " + strconv.Itoa(page) + " 5/2/09 10:37:39",
		)
	}

	t.Run("recurring bottom footer is dropped", func(t *testing.T) {
		t.Parallel()
		pages := [][]htmlBlock{
			{bodyP("para one"), footer(1)},
			{bodyP("para two"), footer(2)},
			{bodyP("para three"), footer(3)},
		}
		got := removeProofSlugLines(pages)
		require.Equal(t, [][]htmlBlock{
			{bodyP("para one")},
			{bodyP("para two")},
			{bodyP("para three")},
		}, got)
	})

	t.Run("recurring top header is dropped", func(t *testing.T) {
		t.Parallel()
		pages := [][]htmlBlock{
			{footer(1), bodyP("para one")},
			{footer(2), bodyP("para two")},
			{footer(3), bodyP("para three")},
		}
		got := removeProofSlugLines(pages)
		require.Equal(t, [][]htmlBlock{
			{bodyP("para one")},
			{bodyP("para two")},
			{bodyP("para three")},
		}, got)
	})

	t.Run("below the recurrence threshold nothing is dropped", func(t *testing.T) {
		t.Parallel()
		pages := [][]htmlBlock{
			{bodyP("para one"), footer(1)},
			{bodyP("para two"), footer(2)},
		}
		got := removeProofSlugLines(pages)
		require.Equal(t, pages, got)
	})

	t.Run("a shape match in the middle of a page is left alone", func(t *testing.T) {
		t.Parallel()
		pages := [][]htmlBlock{
			{bodyP("para one"), footer(1), bodyP("para one tail")},
			{bodyP("para two"), footer(2), bodyP("para two tail")},
			{bodyP("para three"), footer(3), bodyP("para three tail")},
		}
		got := removeProofSlugLines(pages)
		require.Equal(t, pages, got)
	})

	t.Run("empty page list is a no-op", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, removeProofSlugLines(nil))
	})

	t.Run("a page whose only block matches is fully emptied", func(t *testing.T) {
		t.Parallel()
		pages := [][]htmlBlock{
			{footer(1)},
			{footer(2)},
			{footer(3)},
		}
		got := removeProofSlugLines(pages)
		require.Equal(t, [][]htmlBlock{{}, {}, {}}, got)
	})
}
