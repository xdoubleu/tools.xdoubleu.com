//nolint:testpackage // testing unexported service helpers
package services

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	kepubpkg "github.com/pgaskin/kepubify/v4/kepub"
	"github.com/stretchr/testify/require"
)

// convertToEPUB runs goPDFConverter with no catalog metadata and returns the
// EPUB path.
func convertToEPUB(t *testing.T, pdfPath string) string {
	t.Helper()
	return convertToEPUBWithCatalog(t, pdfPath, "", nil)
}

// convertToEPUBWithCatalog runs goPDFConverter with catalog title/authors.
func convertToEPUBWithCatalog(
	t *testing.T, pdfPath, catalogTitle string, catalogAuthors []string,
) string {
	t.Helper()
	outPath := filepath.Join(t.TempDir(), "out.epub")
	err := goPDFConverter(
		context.Background(), pdfPath, outPath, "00000000-0000-0000-0000-000000000001",
		catalogTitle, catalogAuthors,
	)
	require.NoError(t, err)
	return outPath
}

const indexXHTMLEntry = "OEBPS/index.xhtml"

func readZipEntry(t *testing.T, zipPath string) []byte {
	t.Helper()
	zr, err := zip.OpenReader(zipPath)
	require.NoError(t, err)
	defer func() { _ = zr.Close() }()

	for _, f := range zr.File {
		if f.Name == indexXHTMLEntry {
			rc, openErr := f.Open()
			require.NoError(t, openErr)
			defer func() { _ = rc.Close() }()
			data, readErr := io.ReadAll(rc)
			require.NoError(t, readErr)
			return data
		}
	}
	t.Fatalf("zip entry %s not found in %s", indexXHTMLEntry, zipPath)
	return nil
}

func readZipEntryNamed(t *testing.T, zipPath, name string) []byte {
	t.Helper()
	zr, err := zip.OpenReader(zipPath)
	require.NoError(t, err)
	defer func() { _ = zr.Close() }()

	for _, f := range zr.File {
		if f.Name == name {
			rc, openErr := f.Open()
			require.NoError(t, openErr)
			defer func() { _ = rc.Close() }()
			data, readErr := io.ReadAll(rc)
			require.NoError(t, readErr)
			return data
		}
	}
	t.Fatalf("zip entry %s not found in %s", name, zipPath)
	return nil
}

func zipEntryNames(t *testing.T, zipPath string) []string {
	t.Helper()
	zr, err := zip.OpenReader(zipPath)
	require.NoError(t, err)
	defer func() { _ = zr.Close() }()

	names := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	return names
}

// extractedBlock is a text block (tag + text) or an image ("img" + src).
type extractedBlock struct {
	tag  string
	text string
}

// extractBlocks parses index.xhtml into ordered blocks, independent of
// x/net/html's reserialization. RE2 has no backreferences, so each tag is
// matched separately and merged by position.
func extractBlocks(t *testing.T, xhtmlDoc string) []extractedBlock {
	t.Helper()

	type positioned struct {
		pos   int
		block extractedBlock
	}
	var found []positioned

	for _, tag := range []string{"h1", "h2", "p"} {
		re := regexp.MustCompile(`(?s)<` + tag + `[^>]*>(.*?)</` + tag + `>`)
		for _, m := range re.FindAllStringSubmatchIndex(xhtmlDoc, -1) {
			found = append(found, positioned{
				pos:   m[0],
				block: extractedBlock{tag: tag, text: unescapeXML(xhtmlDoc[m[2]:m[3]])},
			})
		}
	}

	imgRe := regexp.MustCompile(`<img src="([^"]*)"[^>]*/?>`)
	for _, m := range imgRe.FindAllStringSubmatchIndex(xhtmlDoc, -1) {
		found = append(found, positioned{
			pos:   m[0],
			block: extractedBlock{tag: "img", text: xhtmlDoc[m[2]:m[3]]},
		})
	}

	sort.Slice(found, func(i, j int) bool { return found[i].pos < found[j].pos })

	blocks := make([]extractedBlock, len(found))
	for i, f := range found {
		blocks[i] = f.block
	}
	return blocks
}

func unescapeXML(s string) string {
	return strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">").Replace(s)
}

func blockTexts(blocks []extractedBlock, tag string) []string {
	var out []string
	for _, b := range blocks {
		if b.tag == tag {
			out = append(out, b.text)
		}
	}
	return out
}

func requireValidKEPUB(t *testing.T, epubPath string) {
	t.Helper()
	epubData, err := os.ReadFile(epubPath)
	require.NoError(t, err)

	zr, err := zip.NewReader(bytes.NewReader(epubData), int64(len(epubData)))
	require.NoError(t, err)

	var buf bytes.Buffer
	err = kepubpkg.NewConverter().Convert(context.Background(), &buf, zr)
	require.NoError(t, err)
	require.NotEmpty(t, buf.Bytes())
}

func TestGoPDFConverter_TwoColumn(t *testing.T) {
	t.Parallel()
	epubPath := convertToEPUB(t, makeTwoColumnPDF(t))
	requireValidKEPUB(t, epubPath)

	xhtml := string(readZipEntry(t, epubPath))
	blocks := extractBlocks(t, xhtml)

	paragraphs := blockTexts(blocks, "p")
	require.Equal(t, []string{
		twoColParaA, twoColParaBJoined, twoColParaC, twoColParaD, twoColParaE,
	}, paragraphs)

	headings := blockTexts(blocks, "h1")
	require.Equal(t, []string{twoColHeading}, headings)

	images := blockTexts(blocks, "img")
	require.Len(t, images, 1)

	idxA := indexOfBlock(blocks, "p", twoColParaA)
	idxImg := indexOfBlock(blocks, "img", images[0])
	idxB := indexOfBlock(blocks, "p", twoColParaBJoined)
	require.Greater(t, idxImg, idxA)
	require.Less(t, idxImg, idxB)
}

func TestGoPDFConverter_SingleColumn(t *testing.T) {
	t.Parallel()
	epubPath := convertToEPUB(t, makeSingleColumnPDF(t))
	requireValidKEPUB(t, epubPath)

	xhtml := string(readZipEntry(t, epubPath))
	blocks := extractBlocks(t, xhtml)

	require.Equal(t, []string{singleColHeading}, blockTexts(blocks, "h1"))
	require.Equal(
		t,
		[]string{singleColParaA, singleColParaB, singleColParaC},
		blockTexts(blocks, "p"),
	)
}

func TestGoPDFConverter_ImageOnly(t *testing.T) {
	t.Parallel()
	epubPath := convertToEPUB(t, makeImageOnlyPDF(t))
	requireValidKEPUB(t, epubPath)

	names := zipEntryNames(t, epubPath)
	found := false
	for _, n := range names {
		if strings.HasSuffix(n, ".png") {
			found = true
		}
	}
	require.True(t, found, "expected an embedded PNG in %v", names)
}

func TestGoPDFConverter_LogoRepeated(t *testing.T) {
	t.Parallel()
	epubPath := convertToEPUB(t, makeLogoRepeatedPDF(t))
	requireValidKEPUB(t, epubPath)

	names := zipEntryNames(t, epubPath)
	pngCount := 0
	for _, n := range names {
		if strings.HasPrefix(n, "OEBPS/fig-") && strings.HasSuffix(n, ".png") {
			pngCount++
		}
	}
	// The repeated logo dedupes to one image; the sub-50px image never appears.
	require.Equal(t, 1, pngCount)
}

func TestGoPDFConverter_ImageOnlyMultiPageDeduped(t *testing.T) {
	t.Parallel()
	epubPath := convertToEPUB(t, makeImageOnlyMultiPagePDF(t))
	requireValidKEPUB(t, epubPath)

	names := zipEntryNames(t, epubPath)
	pngCount := 0
	for _, n := range names {
		if strings.HasPrefix(n, "OEBPS/fig-") && strings.HasSuffix(n, ".png") {
			pngCount++
		}
	}
	// Four identical blank rasters dedupe to one; the filled page is separate.
	require.Equal(t, 2, pngCount)
}

var (
	imgTagRe  = regexp.MustCompile(`<img\b[^>]*>`)
	altAttrRe = regexp.MustCompile(`\balt="([^"]*)"`)
)

func requireAllImagesHaveAlt(t *testing.T, xhtmlDoc string) {
	t.Helper()
	tags := imgTagRe.FindAllString(xhtmlDoc, -1)
	require.NotEmpty(t, tags, "expected at least one <img> tag in %s", xhtmlDoc)
	for _, tag := range tags {
		m := altAttrRe.FindStringSubmatch(tag)
		require.NotNil(t, m, "img tag missing alt attribute: %s", tag)
		require.NotEmpty(t, m[1], "img tag has empty alt attribute: %s", tag)
	}
}

func TestGoPDFConverter_ImagesHaveAltText(t *testing.T) {
	t.Parallel()

	figEPUB := convertToEPUB(t, makeTwoColumnPDF(t))
	requireAllImagesHaveAlt(t, string(readZipEntry(t, figEPUB)))

	fallbackEPUB := convertToEPUB(t, makeImageOnlyPDF(t))
	requireAllImagesHaveAlt(
		t, string(readZipEntry(t, fallbackEPUB)),
	)
}

// TestGoPDFConverter_ProofSlugFooterFiltered: a recurring page-bottom proof
// slug is dropped while body paragraphs, even one with a date, survive.
func TestGoPDFConverter_ProofSlugFooterFiltered(t *testing.T) {
	t.Parallel()
	epubPath := convertToEPUB(t, makeProofSlugPDF(t))
	requireValidKEPUB(t, epubPath)

	xhtml := string(readZipEntry(t, epubPath))
	blocks := extractBlocks(t, xhtml)
	paragraphs := blockTexts(blocks, "p")

	var want []string
	for page := 1; page <= proofSlugPageCount; page++ {
		want = append(want, fmt.Sprintf(proofSlugBodyParaFmt, page))
		if page == proofSlugPageCount {
			want = append(want, proofSlugDateInBodyPara)
		} else {
			want = append(want, fmt.Sprintf(proofSlugBodyExtraFmt, page))
		}
		want = append(want, fmt.Sprintf(proofSlugBodyClosingFmt, page))
	}
	require.Equal(t, want, paragraphs)

	for _, p := range paragraphs {
		require.NotContains(t, p, "canoe scene ocean scan")
	}
}

// TestGoPDFConverter_PrefersCatalogMetadataOverHeading: catalog title/author
// beat the PDF's first heading and info dict.
func TestGoPDFConverter_PrefersCatalogMetadataOverHeading(t *testing.T) {
	t.Parallel()
	epubPath := convertToEPUBWithCatalog(
		t,
		makeSingleColumnPDF(t),
		"Thinking in Systems: A Primer",
		[]string{"Donella H. Meadows"},
	)

	opf := string(readZipEntryNamed(t, epubPath, "OEBPS/content.opf"))
	require.Contains(t, opf, "<dc:title>Thinking in Systems: A Primer</dc:title>")
	require.Contains(t, opf, "<dc:creator>Donella H. Meadows</dc:creator>")
	require.NotContains(t, opf, singleColHeading)

	// The stable identifier lands in dc:identifier so regenerated KEPUBs keep
	// their identity.
	const wantID = "00000000-0000-0000-0000-000000000001"
	require.Contains(t, opf,
		`<dc:identifier id="pub-id">urn:uuid:`+wantID+"</dc:identifier>")
}

// TestGoPDFConverter_FallsBackWithoutCatalogMetadata: without catalog
// metadata the PDF's heading is the title.
func TestGoPDFConverter_FallsBackWithoutCatalogMetadata(t *testing.T) {
	t.Parallel()
	epubPath := convertToEPUB(t, makeSingleColumnPDF(t))

	opf := string(readZipEntryNamed(t, epubPath, "OEBPS/content.opf"))
	require.Contains(t, opf, "<dc:title>"+singleColHeading+"</dc:title>")
}

func indexOfBlock(blocks []extractedBlock, tag, text string) int {
	for i, b := range blocks {
		if b.tag == tag && b.text == text {
			return i
		}
	}
	return -1
}
