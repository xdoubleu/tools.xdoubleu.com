package positions

import (
	"strconv"
	"strings"

	"github.com/ledongthuc/pdf"
)

const (
	// scanSamples is how many body pages the printed-number scan reads.
	scanSamples = 12
	// scanBodyDiv splits the book in fifths; the middle three are sampled.
	scanBodyDiv = 5
	// scanMinVotes is how many sampled pages must agree on an offset.
	scanMinVotes = 3
	// scanEdgeLines is how many lines at a page's top and bottom (header,
	// footer) can carry its printed number.
	scanEdgeLines = 2
	// maxPrintedNumber rules out years and other large numbers in a header.
	maxPrintedNumber = 5000
)

// detectPrintedLabels derives labels for a PDF without /PageLabels from the
// printed numbers in the headers and footers of a few body pages: the offset
// most sampled pages agree on. Nil when none does.
func detectPrintedLabels(reader *pdf.Reader) *pdfLabels {
	pages := reader.NumPage()
	if pages < scanMinVotes {
		return nil
	}
	votes := map[int]int{}
	read := 0
	for _, p := range samplePages(pages) {
		text, err := reader.Page(p).GetPlainText(nil)
		if err != nil {
			continue
		}
		read++
		seen := map[int]bool{}
		for _, n := range edgeNumbers(text) {
			if off := n - p; !seen[off] {
				seen[off] = true
				votes[off]++
			}
		}
	}
	best, bestVotes := 0, 0
	for off, v := range votes {
		if v > bestVotes || (v == bestVotes && off < best) {
			best, bestVotes = off, v
		}
	}
	if bestVotes < scanMinVotes || bestVotes*2 < read {
		return nil
	}
	// Printed page best+p shows on PDF page p; pages before page 1 have none.
	first := max(1, 1-best)
	return &pdfLabels{
		ranges: []labelRange{{
			index: first - 1, style: pdfStyleDecimal, prefix: "", start: first + best,
		}},
		pages: pages,
	}
}

// samplePages are 1-based pages spread over the middle of the book, where
// front and back matter don't interfere.
func samplePages(pages int) []int {
	lo, hi := pages/scanBodyDiv+1, pages*(scanBodyDiv-1)/scanBodyDiv
	if hi-lo+1 <= scanSamples {
		out := make([]int, 0, max(hi-lo+1, 0))
		for p := lo; p <= hi; p++ {
			out = append(out, p)
		}
		return out
	}
	out := make([]int, 0, scanSamples)
	for i := range scanSamples {
		out = append(out, lo+i*(hi-lo)/(scanSamples-1))
	}
	return out
}

// edgeNumbers are the numbers standing first or last on the page's top and
// bottom lines.
func edgeNumbers(text string) []int {
	var lines []string
	for l := range strings.SplitSeq(text, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	var edge []string
	for i, l := range lines {
		if i < scanEdgeLines || i >= len(lines)-scanEdgeLines {
			edge = append(edge, l)
		}
	}
	var out []int
	for _, l := range edge {
		f := strings.Fields(l)
		for _, w := range []string{f[0], f[len(f)-1]} {
			if n, err := strconv.Atoi(w); err == nil && n > 0 && n <= maxPrintedNumber {
				out = append(out, n)
			}
		}
	}
	return out
}
