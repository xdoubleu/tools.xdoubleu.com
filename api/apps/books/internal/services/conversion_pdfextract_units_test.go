//nolint:testpackage // testing unexported service helpers
package services

import (
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
