package services

import (
	"archive/zip"
	"bytes"
	"context"
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

// convertToEPUB runs goPDFConverter end-to-end and returns the path to the
// produced EPUB.
func convertToEPUB(t *testing.T, pdfPath string) string {
	t.Helper()
	outPath := filepath.Join(t.TempDir(), "out.epub")
	err := goPDFConverter(context.Background(), pdfPath, outPath)
	require.NoError(t, err)
	return outPath
}

// readZipEntry returns the bytes of a named entry from a zip file.
func readZipEntry(t *testing.T, zipPath, name string) []byte {
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

// extractedBlock is one block of the extracted XHTML body, in document
// order — either a text block (tag + text) or an image (tag "img" + the src
// filename in text).
type extractedBlock struct {
	tag  string
	text string
}

// extractBlocks parses the OEBPS/index.xhtml produced by the conversion into
// an ordered list of blocks, so tests can assert exact structure/order
// without depending on x/net/html's exact whitespace/attribute
// reserialization. RE2 has no backreferences, so each tag is matched
// separately and the results merged by their position in the document.
func extractBlocks(t *testing.T, xhtmlDoc string) []extractedBlock {
	t.Helper()

	type positioned struct {
		pos   int
		block extractedBlock
	}
	var found []positioned

	for _, tag := range []string{"h1", "h2", "p"} {
		re := regexp.MustCompile(`(?s)<` + tag + `>(.*?)</` + tag + `>`)
		for _, m := range re.FindAllStringSubmatchIndex(xhtmlDoc, -1) {
			found = append(found, positioned{
				pos:   m[0],
				block: extractedBlock{tag: tag, text: unescapeXML(xhtmlDoc[m[2]:m[3]])},
			})
		}
	}

	imgRe := regexp.MustCompile(`<img src="([^"]*)"\s*/?>`)
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

	xhtml := string(readZipEntry(t, epubPath, "OEBPS/index.xhtml"))
	blocks := extractBlocks(t, xhtml)

	paragraphs := blockTexts(blocks, "p")
	require.Equal(t, []string{
		twoColParaA, twoColParaBJoined, twoColParaC, twoColParaD, twoColParaE,
	}, paragraphs)

	headings := blockTexts(blocks, "h1")
	require.Equal(t, []string{twoColHeading}, headings)

	images := blockTexts(blocks, "img")
	require.Len(t, images, 1)

	// The figure must sit between paragraph A and paragraph B (the
	// hyphenated one), per the placement rule.
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

	xhtml := string(readZipEntry(t, epubPath, "OEBPS/index.xhtml"))
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
	// The logo repeats on all 3 pages and must dedupe to a single kept
	// image; the sub-50px image must never appear.
	require.Equal(t, 1, pngCount)
}

func indexOfBlock(blocks []extractedBlock, tag, text string) int {
	for i, b := range blocks {
		if b.tag == tag && b.text == text {
			return i
		}
	}
	return -1
}
