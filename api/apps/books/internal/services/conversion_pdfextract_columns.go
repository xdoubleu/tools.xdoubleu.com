package services

import (
	"math"
	"slices"
	"sort"
)

// gutterYBins/gutterXBins discretize the page into a grid used to find the
// vertical gutter between columns (step 2 of the text algorithm). Fine
// enough to resolve a typical ~10-20pt column gutter on a ~600x800pt page.
const (
	gutterYBins            = 100
	gutterXBins            = 400
	gutterMinEmptyFraction = 0.8
	// gutterMinWidthFraction/gutterMinCharWidths: a gutter must be at least
	// 2% of the page and a few characters wide. An index sets its columns
	// only ~2.8% apart; a fixed 4% missed it.
	gutterMinWidthFraction = 0.02
	gutterMinCharWidths    = 1.5
	gutterMidLow           = 0.35
	gutterMidHigh          = 0.65
	// midpointDivisor halves a (left, right) or (start, end) pair to find its
	// center — used for both the gutter's midpoint and column-assignment
	// midpoints below.
	midpointDivisor = 2
	// roundHalfUp nudges a truncating int conversion into round-half-up.
	roundHalfUp = 0.5
)

// findGutter locates the widest vertical strip of the page that contains no
// character boxes across at least 80% of the page's text rows. The page is
// two-column only if that gutter is at least 2% of the page width and two
// median character widths, and its midpoint falls between 35% and 65% of
// the page width.
func findGutter(
	chars []pdfChar,
	pageWidth, pageHeight float64,
) (float64, float64, bool) {
	if pageWidth <= 0 || pageHeight <= 0 || len(chars) == 0 {
		return 0, 0, false
	}

	occupied := make([][]bool, gutterYBins)
	for i := range occupied {
		occupied[i] = make([]bool, gutterXBins)
	}

	xBinWidth := pageWidth / gutterXBins
	yBinHeight := pageHeight / gutterYBins

	for _, c := range chars {
		yStart := clampBin(int((pageHeight-c.top)/yBinHeight), gutterYBins)
		yEnd := clampBin(int((pageHeight-c.bottom)/yBinHeight), gutterYBins)
		if yStart > yEnd {
			yStart, yEnd = yEnd, yStart
		}
		xStart := clampBin(int(c.left/xBinWidth), gutterXBins)
		xEnd := clampBin(int(c.right/xBinWidth), gutterXBins)
		for y := yStart; y <= yEnd; y++ {
			for x := xStart; x <= xEnd; x++ {
				occupied[y][x] = true
			}
		}
	}

	bestStart, bestEnd := widestEmptyRun(occupied)
	if bestStart == -1 {
		return 0, 0, false
	}

	gutterLeft := float64(bestStart) * xBinWidth
	gutterRight := float64(bestEnd) * xBinWidth
	width := gutterRight - gutterLeft
	if width < gutterMinWidthFraction*pageWidth ||
		width < gutterMinCharWidths*medianCharWidth(chars) {
		return 0, 0, false
	}

	return gutterLeft, gutterRight, true
}

func clampBin(v, upper int) int {
	if v < 0 {
		return 0
	}
	if v >= upper {
		return upper - 1
	}
	return v
}

// widestEmptyRun returns the [start,end) x-bin range of the widest centred
// run (see centralRun) of columns empty in most rows holding text (see
// emptyColumns). Only centred runs compete, so a wide page margin can't
// outrank a narrow gutter.
func widestEmptyRun(occupied [][]bool) (int, int) {
	xBins := len(occupied[0])
	emptyEnough := emptyColumns(occupied)
	if emptyEnough == nil {
		return -1, -1
	}

	bestStart, bestEnd := -1, -1
	curStart := -1
	for x := 0; x <= xBins; x++ {
		open := x < xBins && emptyEnough[x]
		switch {
		case open && curStart == -1:
			curStart = x
		case !open && curStart != -1:
			if centralRun(curStart, x, xBins) &&
				(bestStart == -1 || x-curStart > bestEnd-bestStart) {
				bestStart, bestEnd = curStart, x
			}
			curStart = -1
		}
	}
	return bestStart, bestEnd
}

// colStats holds the per-column typographic constants used by the
// paragraph-break heuristics: the column's right text margin and its modal
// (most common) line start, used to detect a short line or an indent.
type colStats struct {
	rightEdge   float64
	modalXStart float64
}

func computeColStats(lines []pdfLine) colStats {
	if len(lines) == 0 {
		return colStats{rightEdge: 0, modalXStart: 0}
	}

	rightEdge := lines[0].right
	freq := map[float64]int{}
	for _, l := range lines {
		rightEdge = max(rightEdge, l.right)
		freq[roundTo(l.left, 1)]++
	}

	modalXStart, bestCount := lines[0].left, 0
	for v, count := range freq {
		if count > bestCount || (count == bestCount && v < modalXStart) {
			modalXStart, bestCount = v, count
		}
	}

	return colStats{rightEdge: rightEdge, modalXStart: modalXStart}
}

func roundTo(v float64, step float64) float64 {
	return float64(int(v/step+roundHalfUp)) * step
}

// assignColumns splits lines into left/right columns (step 3: reading
// order), applies each column's stats to every line in it, and sorts each
// column top-to-bottom. On a single-column page all lines are the "left"
// column and right is empty.
func assignColumns(
	lines []pdfLine, gutterLeft, gutterRight float64, twoColumn bool,
) ([]pdfLine, []pdfLine) {
	var left, right []pdfLine

	if !twoColumn {
		left = append(left, lines...)
	} else {
		gutterMid := (gutterLeft + gutterRight) / midpointDivisor
		leftTop := math.Inf(-1)
		for _, l := range lines {
			if l.right <= gutterMid {
				leftTop = max(leftTop, l.top)
			}
		}
		for _, l := range lines {
			// A line straddling the gutter (a title) or above the whole
			// left column (a recto page's running header) reads with the
			// left column, which sorts it first.
			if l.left < gutterMid || l.bottom > leftTop {
				left = append(left, l)
			} else {
				right = append(right, l)
			}
		}
	}

	applyColStats(left, 0)
	applyColStats(right, 1)
	sortLinesTopToBottom(left)
	sortLinesTopToBottom(right)

	return left, right
}

func applyColStats(lines []pdfLine, col int) {
	stats := computeColStats(lines)
	for i := range lines {
		lines[i].colRightEdge = stats.rightEdge
		lines[i].colModalXStart = stats.modalXStart
		lines[i].col = col
	}
}

func sortLinesTopToBottom(lines []pdfLine) {
	sort.SliceStable(
		lines,
		func(i, j int) bool { return lines[i].yMid() > lines[j].yMid() },
	)
}

// emptyColumns reports, per x bin, whether it is empty in at least
// gutterMinEmptyFraction of the rows holding text; nil when no row does.
func emptyColumns(occupied [][]bool) []bool {
	var textRows [][]bool
	for _, row := range occupied {
		if slices.Contains(row, true) {
			textRows = append(textRows, row)
		}
	}
	if len(textRows) == 0 {
		return nil
	}
	emptyEnough := make([]bool, len(occupied[0]))
	for x := range emptyEnough {
		empty := 0
		for _, row := range textRows {
			if !row[x] {
				empty++
			}
		}
		emptyEnough[x] = float64(empty)/float64(len(textRows)) >= gutterMinEmptyFraction
	}
	return emptyEnough
}

// centralRun reports whether the [start,end) bin run's midpoint falls
// between gutterMidLow and gutterMidHigh of the page width.
func centralRun(start, end, bins int) bool {
	mid := float64(start+end) / midpointDivisor / float64(bins)
	return mid >= gutterMidLow && mid <= gutterMidHigh
}
