//nolint:testpackage // testing unexported page number scan
package positions

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// numberedPDF has pages pages; page p's footer reads p+offset, or nothing
// for pages the footer skips (number <= 0).
func numberedPDF(pages, offset int) []byte {
	var buf bytes.Buffer
	offsets := []int{}
	obj := func(body string) {
		offsets = append(offsets, buf.Len())
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", len(offsets), body)
	}
	buf.WriteString("%PDF-1.4\n")
	obj("<< /Type /Catalog /Pages 2 0 R >>")
	kids := ""
	for i := range pages {
		kids += fmt.Sprintf("%d 0 R ", 4+i*2)
	}
	obj(fmt.Sprintf("<< /Type /Pages /Count %d /Kids [%s] >>", pages, kids))
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	for p := 1; p <= pages; p++ {
		obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] "+
			"/Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", 5+(p-1)*2))
		stream := "BT /F1 12 Tf 20 180 Td (Some chapter text) Tj ET\n"
		if n := p + offset; n > 0 {
			stream += fmt.Sprintf("BT /F1 12 Tf 90 20 Td (%d) Tj ET\n", n)
		}
		obj(fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream))
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, o := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(offsets)+1, xref)
	return buf.Bytes()
}

func TestReadPDFLabels_ScansPrintedNumbers(t *testing.T) {
	data := numberedPDF(40, -6)
	l, err := readPDFLabels(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	require.True(t, l.usable())
	assert.Equal(t, 0, l.printedOf(3))
	assert.Equal(t, 1, l.printedOf(7))
	assert.Equal(t, 20, l.printedOf(26))
	at, ok := l.pageOf(1)
	assert.True(t, ok)
	assert.Equal(t, 7, at)
}

func TestReadPDFLabels_ScanPositiveOffset(t *testing.T) {
	data := numberedPDF(40, 2)
	l, err := readPDFLabels(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	require.True(t, l.usable())
	assert.Equal(t, 3, l.printedOf(1))
	at, ok := l.pageOf(10)
	assert.True(t, ok)
	assert.Equal(t, 8, at)
}

func TestReadPDFLabels_ScanNeedsAgreement(t *testing.T) {
	data := numberedPDF(3, 0)
	l, err := readPDFLabels(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	assert.False(t, l.usable(), "too few pages")
	assert.Empty(t, samplePages(0))
	assert.Len(t, samplePages(500), scanSamples)
	assert.Empty(t, edgeNumbers("no numbers here"))
	assert.Equal(t, []int{42, 42}, edgeNumbers("a\n\n42\n"))
}
