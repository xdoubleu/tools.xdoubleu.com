package services

import "sort"

// gutterYBins/gutterXBins discretize the page into a grid used to find the
// vertical gutter between columns (step 2 of the text algorithm). Fine
// enough to resolve a typical ~10-20pt column gutter on a ~600x800pt page.
const (
	gutterYBins            = 100
	gutterXBins            = 200
	gutterMinEmptyFraction = 0.8
	gutterMinWidthFraction = 0.04
	gutterMidLow           = 0.35
	gutterMidHigh          = 0.65
)

// findGutter locates the widest vertical strip of the page that contains no
// character boxes across at least 80% of the page height. The page is
// two-column only if that gutter is at least 4% of the page width and its
// midpoint falls between 35% and 65% of the page width.
func findGutter(
	chars []pdfChar,
	pageWidth, pageHeight float64,
) (left, right float64, twoColumn bool) {
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
	if gutterRight-gutterLeft < gutterMinWidthFraction*pageWidth {
		return 0, 0, false
	}

	midFrac := (gutterLeft + gutterRight) / 2 / pageWidth
	if midFrac < gutterMidLow || midFrac > gutterMidHigh {
		return 0, 0, false
	}

	return gutterLeft, gutterRight, true
}

func clampBin(v, max int) int {
	if v < 0 {
		return 0
	}
	if v >= max {
		return max - 1
	}
	return v
}

// widestEmptyRun returns the [start,end) x-bin range of the widest run of
// columns empty for at least gutterMinEmptyFraction of the page's rows.
func widestEmptyRun(occupied [][]bool) (start, end int) {
	xBins := len(occupied[0])
	yBins := len(occupied)

	emptyEnough := make([]bool, xBins)
	for x := 0; x < xBins; x++ {
		empty := 0
		for y := 0; y < yBins; y++ {
			if !occupied[y][x] {
				empty++
			}
		}
		emptyEnough[x] = float64(empty)/float64(yBins) >= gutterMinEmptyFraction
	}

	bestStart, bestEnd := -1, -1
	curStart := -1
	for x := 0; x <= xBins; x++ {
		open := x < xBins && emptyEnough[x]
		switch {
		case open && curStart == -1:
			curStart = x
		case !open && curStart != -1:
			if bestStart == -1 || x-curStart > bestEnd-bestStart {
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
		return colStats{}
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
	return float64(int(v/step+0.5)) * step
}

// assignColumns splits lines into left/right columns (step 3: reading
// order), applies each column's stats to every line in it, and sorts each
// column top-to-bottom. On a single-column page all lines are the "left"
// column and right is empty.
func assignColumns(
	lines []pdfLine, gutterLeft, gutterRight float64, twoColumn bool,
) (left, right []pdfLine) {
	if !twoColumn {
		left = append(left, lines...)
	} else {
		gutterMid := (gutterLeft + gutterRight) / 2
		for _, l := range lines {
			if l.xMid() < gutterMid {
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
