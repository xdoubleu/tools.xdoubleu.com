//nolint:testpackage // testing unexported service helpers
package services

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyOutline(t *testing.T) {
	t.Parallel()

	img := imageBlock("fig-0.png", "Figure")
	pages := [][]htmlBlock{
		{flowBlock("PART ONE"), flowBlock("System Structure and Behavior")},
		{
			flowBlock("— ONE —"),
			flowBlock("The Basics"),
			flowBlock("Body text starts here."),
		},
		{
			img,
			flowBlock("More Than the Sum of Its Parts"),
			flowBlock("A system is more."),
		},
		{flowBlock("Body text continues without a heading.")},
	}
	outline := []outlineEntry{
		{title: "PART ONE System Structure and Behavior", level: 1, page: 0},
		{title: "Chapter One The Basics", level: 2, page: 1},
		{title: "More Than the Sum of Its Parts", level: 3, page: 2},
		{title: "Bathtubs 101&#8212;Understanding Stocks", level: 3, page: 3},
	}

	got := applyOutline(pages, outline)
	type heading struct {
		text  string
		level int
	}
	var headings []heading
	for _, blocks := range got {
		for _, b := range blocks {
			if b.tocLevel > 0 {
				headings = append(headings, heading{b.text, b.tocLevel})
			}
		}
	}
	assert.Equal(t, []heading{
		{"PART ONE System Structure and Behavior", 1},
		{"ONE The Basics", 2},
		{"More Than the Sum of Its Parts", 3},
		{"Bathtubs 101—Understanding Stocks", 3},
	}, headings)
	assert.Len(t, got[0], 1, "the two title lines merge into one heading")
	assert.Equal(t, "Body text starts here.", got[1][1].text)
	assert.Equal(t, imgTag, got[2][0].tag)
	assert.Equal(t, "Bathtubs 101—Understanding Stocks", got[3][0].text,
		"an entry with no heading on its page gets one")
}

// makeOutlinePDF: a Part page and a chapter page with a section heading, a
// bookmark for each, and a heading-sized line that isn't in the outline.
func makeOutlinePDF(t *testing.T) string {
	t.Helper()
	pdf := newFixturePDF()
	pdf.Bookmark("PART ONE System Structure", 0, 0)
	fixtureText(pdf, 250, 300, fixtureHeadingSize, "PART ONE")
	fixtureText(pdf, 200, 330, fixtureHeadingSize, "System Structure")

	pdf.AddPage()
	pdf.Bookmark("Chapter One The Basics", 1, 0)
	fixtureText(pdf, fixtureLeftX, 80, fixtureHeadingSize, "The Basics")
	pdf.Bookmark("More Than the Sum", 2, 0)
	fixtureText(
		pdf,
		fixtureLeftX,
		80+fixtureBreakDY,
		fixtureBodySize,
		"More Than the Sum",
	)
	fixtureText(pdf, fixtureLeftX, 80+2*fixtureBreakDY, fixtureBodySize, singleColParaA)
	fixtureText(
		pdf,
		fixtureLeftX,
		80+3*fixtureBreakDY,
		fixtureHeadingSize,
		"THE WAY OUT",
	)
	fixtureText(pdf, fixtureLeftX, 80+4*fixtureBreakDY, fixtureBodySize, singleColParaB)
	return savePDF(t, pdf, "outline.pdf")
}

func TestGoPDFConverter_OutlineDrivesTOC(t *testing.T) {
	t.Parallel()
	epubPath := convertToEPUB(t, makeOutlinePDF(t))
	requireValidKEPUB(t, epubPath)

	nav := string(readZipEntryNamed(t, epubPath, "OEBPS/nav.xhtml"))
	nav, _, _ = strings.Cut(nav, `epub:type="page-list"`)
	titles := regexp.MustCompile(`<a [^>]*>([^<]*)</a>`).FindAllStringSubmatch(nav, -1)
	var got []string
	for _, m := range titles {
		got = append(got, m[1])
	}
	require.Equal(
		t,
		[]string{"PART ONE System Structure", "The Basics", "More Than the Sum"},
		got,
	)
	require.Regexp(
		t,
		`(?s)<li>.*PART ONE System Structure.*<ol>.*The Basics.*<ol>.*More Than the Sum`,
		nav,
	)

	body := pageBreakRe.ReplaceAllString(string(readZipEntry(t, epubPath)), "")
	require.Regexp(t, `<h1[^>]*>PART ONE System Structure</h1>`, body)
	require.Regexp(t, `<h2[^>]*>The Basics</h2>`, body)
	require.Regexp(t, `<h3[^>]*>More Than the Sum</h3>`, body)
	require.Regexp(t, `<h4>THE WAY OUT</h4>`, body)
}
