//nolint:testpackage // testing unexported service helpers
package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	vectorParaAbove = "Imagine a bathtub filled with water, drain plugged up."
	vectorCaption   = "Figure 1. Water in a tub, with one inflow and one outflow."
	vectorBoxed1    = "A note on reading graphs: behavior over time graphs show the"
	vectorBoxed2    = "trend of a variable, and the points at which that trend changes"
	vectorBoxed3    = "shape or direction matter more than the numbers along the axes."
)

// makeVectorFigurePDF: a paragraph, a vector stock-and-flow diagram and graph
// frame with labels inside and tick labels outside, its caption, and a
// ruled box around a paragraph of prose (a sidebar, not a figure).
func makeVectorFigurePDF(t *testing.T) string {
	t.Helper()
	pdf := newFixturePDF()
	fixtureText(pdf, fixtureLeftX, 60, fixtureBodySize, vectorParaAbove)

	// The diagram: stock box, pipes, and a graph frame below it.
	pdf.SetLineWidth(1)
	pdf.Rect(200, 100, 80, 40, "D")
	pdf.Line(120, 120, 200, 120)
	pdf.Line(280, 120, 360, 120)
	pdf.Rect(150, 160, 220, 120, "D")
	pdf.Line(150, 220, 370, 240)
	fixtureText(pdf, 222, 124, fixtureBodySize, "stock")
	fixtureText(pdf, 125, 115, fixtureBodySize, "inflow")
	fixtureText(pdf, 300, 115, fixtureBodySize, "outflow")
	// Two labels sharing a baseline read as one six-word line.
	fixtureText(pdf, 120, 150, fixtureBodySize, "heat from furnace")
	fixtureText(pdf, 290, 150, fixtureBodySize, "heat to outside")
	for i, tick := range []string{"50", "40", "30", "20", "10"} {
		fixtureText(pdf, 135, 165+float64(i)*28, fixtureBodySize, tick)
	}
	fixtureText(pdf, 150, 290, fixtureBodySize, "0    2    4    6    8    10")
	fixtureText(pdf, 240, 302, fixtureBodySize, "minutes")
	fixtureText(pdf, fixtureLeftX, 320, fixtureBodySize, vectorCaption)

	pdf.Rect(fixtureLeftX-10, 360, 500, 60, "D")
	fixtureText(pdf, fixtureLeftX, 378, fixtureBodySize, vectorBoxed1)
	fixtureText(pdf, fixtureLeftX, 392, fixtureBodySize, vectorBoxed2)
	fixtureText(pdf, fixtureLeftX, 406, fixtureBodySize, vectorBoxed3)
	return savePDF(t, pdf, "vector-figure.pdf")
}

func TestGoPDFConverter_VectorFigureRendered(t *testing.T) {
	t.Parallel()
	epubPath := convertToEPUB(t, makeVectorFigurePDF(t))
	requireValidKEPUB(t, epubPath)

	blocks := extractBlocks(t, string(readZipEntry(t, epubPath)))
	var tags, texts []string
	for _, b := range blocks {
		tags = append(tags, b.tag)
		texts = append(texts, b.text)
	}
	joined := strings.Join(texts, "\n")
	for _, text := range texts {
		labels := []string{"stock", "inflow", "outflow", "minutes", "50", "0", "heat"}
		for _, label := range labels {
			require.NotContains(t, strings.Fields(text)[:1], label,
				"diagram label leaked into text: %q", text)
		}
	}
	require.Equal(t, []string{"p", "img", "p", "p"}, tags, joined)
	require.Equal(t, vectorParaAbove, texts[0])
	require.Equal(t, vectorCaption, texts[2])
	require.Equal(t, vectorBoxed1+" "+vectorBoxed2+" "+vectorBoxed3, texts[3])
}

func TestGoPDFConverter_FigureAltFromCaption(t *testing.T) {
	t.Parallel()
	doc := string(readZipEntry(t, convertToEPUB(t, makeVectorFigurePDF(t))))
	require.Contains(t, doc, `alt="`+vectorCaption+`"`)
}

// makePanelFigurePDF: two stacked graph panels of one figure, each with a
// legend beside it and a tick row between, a body line of one word below
// the figure's reach, then the caption.
func makePanelFigurePDF(t *testing.T) string {
	t.Helper()
	pdf := newFixturePDF()
	fixtureText(pdf, fixtureLeftX, 60, fixtureBodySize, vectorParaAbove)
	for i, legend := range []string{"A: Extraction rate", "B: Capital stock"} {
		top := 100 + float64(i)*110
		pdf.Rect(200, top, 180, 80, "D")
		pdf.Line(200, top+60, 380, top+20)
		fixtureText(pdf, 95, top+8, fixtureBodySize, legend)
		fixtureText(pdf, 200, top+92, fixtureBodySize, "0    25    50    75    100")
	}
	fixtureText(pdf, fixtureLeftX, 360, fixtureBodySize, "depleted.")
	fixtureText(pdf, fixtureLeftX, 390, fixtureBodySize, vectorCaption)
	return savePDF(t, pdf, "panel-figure.pdf")
}

func TestGoPDFConverter_PanelFigureWithSideLegends(t *testing.T) {
	t.Parallel()
	blocks := extractBlocks(
		t,
		string(readZipEntry(t, convertToEPUB(t, makePanelFigurePDF(t)))),
	)
	var tags, texts []string
	for _, b := range blocks {
		tags = append(tags, b.tag)
		texts = append(texts, b.text)
	}
	require.Equal(t, []string{"p", "img", "p", "p"}, tags, strings.Join(texts, "\n"))
	require.Equal(t, "depleted.", texts[2])
}

// makeTwoFiguresPDF: two captioned figures on one page, drawn bottom figure
// first, the upper caption wrapping onto a short second line just above the
// lower figure's labels.
func makeTwoFiguresPDF(t *testing.T) string {
	t.Helper()
	pdf := newFixturePDF()
	fixtureText(pdf, fixtureLeftX, 60, fixtureBodySize, vectorParaAbove)
	for _, top := range []float64{330, 100} {
		pdf.Rect(200, top, 180, 80, "D")
		pdf.Line(200, top+60, 380, top+20)
		fixtureText(pdf, 200, top+92, fixtureBodySize, "0    25    50    75    100")
	}
	fixtureText(
		pdf,
		fixtureLeftX,
		215,
		fixtureBodySize,
		"Figure 1. The response of inventory to the same increase in demand "+
			"with a shortened percep-",
	)
	fixtureText(pdf, fixtureLeftX, 227, fixtureBodySize, "tion delay.")
	fixtureText(pdf, fixtureLeftX, 445, fixtureBodySize, "Figure 2. The second graph.")
	return savePDF(t, pdf, "two-figures.pdf")
}

func TestGoPDFConverter_TwoFiguresInReadingOrder(t *testing.T) {
	t.Parallel()
	blocks := extractBlocks(
		t,
		string(readZipEntry(t, convertToEPUB(t, makeTwoFiguresPDF(t)))),
	)
	var tags, texts []string
	for _, b := range blocks {
		tags = append(tags, b.tag)
		texts = append(texts, b.text)
	}
	require.Equal(
		t,
		[]string{"p", "img", "p", "img", "p"},
		tags,
		strings.Join(texts, "\n"),
	)
	require.Equal(t,
		"Figure 1. The response of inventory to the same increase in demand "+
			"with a shortened perception delay.", texts[2])
}

// TestGoPDFConverter_ShadedSidebarStaysText: a filled box behind a one-line
// question is a sidebar, not a figure.
func TestGoPDFConverter_ShadedSidebarStaysText(t *testing.T) {
	t.Parallel()
	pdf := newFixturePDF()
	fixtureText(pdf, fixtureLeftX, 60, fixtureBodySize, vectorParaAbove)
	pdf.SetFillColor(220, 220, 220)
	pdf.Rect(150, 100, 300, 50, "F")
	fixtureText(pdf, 250, 118, fixtureBodySize, "THINK ABOUT THIS:")
	fixtureText(
		pdf,
		170,
		138,
		fixtureBodySize,
		"If A causes B, is it possible that B also causes A?",
	)
	blocks := extractBlocks(
		t,
		string(readZipEntry(t, convertToEPUB(t, savePDF(t, pdf, "sidebar.pdf")))),
	)
	var texts []string
	for _, b := range blocks {
		require.NotEqual(t, "img", b.tag)
		texts = append(texts, b.text)
	}
	require.Contains(t, strings.Join(texts, " "), "If A causes B")
}

func TestOutsideFigures(t *testing.T) {
	t.Parallel()

	fig := func(left, bottom, right, top float64) rawFigure {
		return rawFigure{png: nil, left: left, top: top, right: right, bottom: bottom}
	}
	inside, outside := fig(20, 20, 40, 40), fig(200, 20, 240, 40)
	got := outsideFigures([]rawFigure{inside, outside}, []rawFigure{fig(10, 10, 100, 100)})
	require.Equal(t, []rawFigure{outside}, got)
}
