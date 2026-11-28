package positions

import (
	"fmt"
	"io"
	"sort"

	"github.com/ledongthuc/pdf"
)

// maxLabelTreeDepth bounds the /PageLabels number tree walk.
const maxLabelTreeDepth = 16

// pdfStyleDecimal is the /S style of arabic page labels; roman and letter
// styles name pages the user can't enter.
const pdfStyleDecimal = "D"

// labelRange is a run of pages numbered alike from index (0-based).
type labelRange struct {
	index  int
	style  string
	prefix string
	start  int
}

// pdfLabels are a PDF's page labels, so a printed page number maps to the PDF
// page that shows it and back (ADR-0031). Only decimal, unprefixed labels
// count as printed pages.
type pdfLabels struct {
	ranges []labelRange
	pages  int
}

// readPDFLabels reads r's /PageLabels, else the printed numbers in its page
// text. A PDF with neither yields nil; a malformed one nil and an error. ledongthuc/pdf panics on malformed objects.
func readPDFLabels(r io.ReaderAt, size int64) (*pdfLabels, error) {
	var labels *pdfLabels
	var err error
	func() {
		defer func() {
			if p := recover(); p != nil {
				err = fmt.Errorf("read pdf page labels: %v", p)
			}
		}()
		labels, err = parsePDFLabels(r, size)
	}()
	return labels, err
}

func parsePDFLabels(r io.ReaderAt, size int64) (*pdfLabels, error) {
	reader, err := pdf.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}
	var ranges []labelRange
	collectLabelRanges(reader.Trailer().Key("Root").Key("PageLabels"), 0, &ranges)
	if len(ranges) == 0 {
		return detectPrintedLabels(reader), nil
	}
	sort.SliceStable(ranges, func(a, b int) bool {
		return ranges[a].index < ranges[b].index
	})
	return &pdfLabels{ranges: ranges, pages: reader.NumPage()}, nil
}

// collectLabelRanges walks a number tree node: leaves hold /Nums pairs of page
// index and label dictionary, inner nodes /Kids.
func collectLabelRanges(node pdf.Value, depth int, out *[]labelRange) {
	if depth > maxLabelTreeDepth {
		return
	}
	nums := node.Key("Nums")
	for i := 0; i+1 < nums.Len(); i += 2 {
		dict := nums.Index(i + 1)
		start := 1
		if s := dict.Key("St"); s.Kind() == pdf.Integer {
			start = int(s.Int64())
		}
		*out = append(*out, labelRange{
			index: int(nums.Index(i).Int64()), style: dict.Key("S").Name(),
			prefix: dict.Key("P").Text(), start: start,
		})
	}
	kids := node.Key("Kids")
	for i := range kids.Len() {
		collectLabelRanges(kids.Index(i), depth+1, out)
	}
}

// end is the 0-based index after range i's last page.
func (l *pdfLabels) end(i int) int {
	if i+1 < len(l.ranges) {
		return l.ranges[i+1].index
	}
	return max(l.pages, l.ranges[i].index+1)
}

func (r labelRange) printed() bool {
	return r.style == pdfStyleDecimal && r.prefix == ""
}

// usable reports whether any page has a printed number.
func (l *pdfLabels) usable() bool {
	if l == nil {
		return false
	}
	for _, r := range l.ranges {
		if r.printed() && r.start >= 0 {
			return true
		}
	}
	return false
}

// printedOf is the printed number of the 1-based PDF page, 0 when it has
// none (front matter, a cover before the first range).
func (l *pdfLabels) printedOf(page int) int {
	idx := page - 1
	i := sort.Search(len(l.ranges), func(i int) bool {
		return l.ranges[i].index > idx
	}) - 1
	if i < 0 || !l.ranges[i].printed() {
		return 0
	}
	n := l.ranges[i].start + idx - l.ranges[i].index
	if n <= 0 {
		return 0
	}
	return n
}

// pageOf is the 1-based PDF page printed page starts on; false when no
// range numbers it.
func (l *pdfLabels) pageOf(printed int) (int, bool) {
	for i, r := range l.ranges {
		if !r.printed() {
			continue
		}
		n := printed - r.start
		if n >= 0 && r.index+n < l.end(i) {
			return r.index + n + 1, true
		}
	}
	return 0, false
}
