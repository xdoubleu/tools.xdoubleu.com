package services

import "sort"

// Step 2: the page grid used to find the column gutter, fine enough for a
// ~10-20pt gutter on a ~600x800pt page.
const (
	gutterYBins            = 100
	gutterXBins            = 200
	gutterMinEmptyFraction = 0.8
	gutterMinWidthFraction = 0.04
	gutterMidLow           = 0.35
	gutterMidHigh          = 0.65
	midpointDivisor        = 2
	roundHalfUp            = 0.5
)

// findGutter finds the widest strip empty across >= 80% of the page height.
// The page is two-column only if it is >= 4% of the width and centered
// between 35% and 65%.
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
	if gutterRight-gutterLeft < gutterMinWidthFraction*pageWidth {
		return 0, 0, false
	}

	midFrac := (gutterLeft + gutterRight) / midpointDivisor / pageWidth
	if midFrac < gutterMidLow || midFrac > gutterMidHigh {
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

// widestEmptyRun returns the [start,end) x-bins of the widest mostly-empty run.
func widestEmptyRun(occupied [][]bool) (int, int) {
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

// colStats holds a column's right margin and modal line start, for detecting
// short lines and indents.
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

// assignColumns splits lines into left/right columns, applies column stats,
// and sorts each top-to-bottom. Single-column pages use left only.
func assignColumns(
	lines []pdfLine, gutterLeft, gutterRight float64, twoColumn bool,
) ([]pdfLine, []pdfLine) {
	var left, right []pdfLine

	if !twoColumn {
		left = append(left, lines...)
	} else {
		gutterMid := (gutterLeft + gutterRight) / midpointDivisor
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
