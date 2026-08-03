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
	"testing"

	kepubpkg "github.com/pgaskin/kepubify/v4/kepub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	xhtml "golang.org/x/net/html"
)

// writeArticleFixture writes index.html (plus any named image files) into a
// fresh temp dir, mirroring buildArticleEPUB's real layout, and returns the
// index.html path.
func writeArticleFixture(
	t *testing.T, body string, images map[string][]byte,
) string {
	t.Helper()
	dir := t.TempDir()
	htmlPath := filepath.Join(dir, "index.html")
	require.NoError(t, os.WriteFile(htmlPath, []byte(body), 0o600))
	for name, data := range images {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0o600))
	}
	return htmlPath
}

// convertToEPUBZip runs goHTMLConverter on inPath and opens the result as a
// zip.Reader.
func convertToEPUBZip(
	t *testing.T, inPath string, meta ArticleMeta,
) *zip.Reader {
	t.Helper()
	outPath := filepath.Join(t.TempDir(), "out.epub")
	err := goHTMLConverter(context.Background(), inPath, outPath, meta)
	require.NoError(t, err)

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	return zr
}

func zipEntryContent(t *testing.T, zr *zip.Reader, name string) string {
	t.Helper()
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		require.NoError(t, err)
		defer func() { _ = rc.Close() }()
		data, err := io.ReadAll(rc)
		require.NoError(t, err)
		return string(data)
	}
	t.Fatalf("zip entry %s not found", name)
	return ""
}

// newTestNode builds a minimal element node for exercising tree-mutation
// helpers directly, without going through xhtml.Parse.
func newTestNode(tag string, attrs []xhtml.Attribute) *xhtml.Node {
	//nolint:exhaustruct // only Type/Data/Attr matter for these unit tests
	return &xhtml.Node{
		Type: xhtml.ElementNode,
		Data: tag,
		Attr: attrs,
	}
}

func TestGoHTMLConverter_MimetypeFirstAndStored(t *testing.T) {
	inPath := writeArticleFixture(t, "<html><body><p>hi</p></body></html>", nil)
	zr := convertToEPUBZip(t, inPath, ArticleMeta{Title: "Test", Authors: nil})

	require.NotEmpty(t, zr.File)
	assert.Equal(t, "mimetype", zr.File[0].Name)
	assert.Equal(t, zip.Store, zr.File[0].Method)
	assert.Equal(t, "application/epub+zip", zipEntryContent(t, zr, "mimetype"))
}

func TestGoHTMLConverter_ContainerPointsAtOPF(t *testing.T) {
	inPath := writeArticleFixture(t, "<html><body><p>hi</p></body></html>", nil)
	zr := convertToEPUBZip(t, inPath, ArticleMeta{Title: "Test", Authors: nil})

	container := zipEntryContent(t, zr, "META-INF/container.xml")
	assert.Contains(t, container, `full-path="OEBPS/content.opf"`)
}

func TestGoHTMLConverter_MetadataInOPF(t *testing.T) {
	inPath := writeArticleFixture(t, "<html><body><p>hi</p></body></html>", nil)
	zr := convertToEPUBZip(t, inPath, ArticleMeta{
		Title:   "My Article",
		Authors: []string{"Alice", "Bob"},
	})

	opf := zipEntryContent(t, zr, "OEBPS/content.opf")
	assert.Contains(t, opf, "<dc:title>My Article</dc:title>")
	assert.Contains(t, opf, "<dc:creator>Alice</dc:creator>")
	assert.Contains(t, opf, "<dc:creator>Bob</dc:creator>")
	assert.Contains(t, opf, "urn:uuid:")
}

func TestGoHTMLConverter_NoAuthorsOmitsCreator(t *testing.T) {
	inPath := writeArticleFixture(t, "<html><body><p>hi</p></body></html>", nil)
	zr := convertToEPUBZip(
		t, inPath, ArticleMeta{Title: "No Authors", Authors: nil},
	)

	opf := zipEntryContent(t, zr, "OEBPS/content.opf")
	assert.NotContains(t, opf, "<dc:creator>")
}

func TestGoHTMLConverter_EmbedsAndManifestsImages(t *testing.T) {
	jpg := []byte{0xFF, 0xD8, 0xFF}
	inPath := writeArticleFixture(
		t,
		`<html><body><img src="img_0.jpg"></body></html>`,
		map[string][]byte{"img_0.jpg": jpg},
	)
	zr := convertToEPUBZip(t, inPath, ArticleMeta{Title: "Img", Authors: nil})

	assert.Equal(t, string(jpg), zipEntryContent(t, zr, "OEBPS/img_0.jpg"))

	opf := zipEntryContent(t, zr, "OEBPS/content.opf")
	assert.Contains(
		t, opf, fmt.Sprintf(`href="img_0.jpg" media-type="%s"`, contentTypeJPEG),
	)

	index := zipEntryContent(t, zr, "OEBPS/index.xhtml")
	assert.Contains(t, index, `src="img_0.jpg"`)
}

func TestGoHTMLConverter_DropsMissingImage(t *testing.T) {
	inPath := writeArticleFixture(
		t, `<html><body><img src="missing.jpg"></body></html>`, nil,
	)
	zr := convertToEPUBZip(t, inPath, ArticleMeta{Title: "Missing", Authors: nil})

	for _, f := range zr.File {
		assert.NotEqual(t, "OEBPS/missing.jpg", f.Name)
	}
	opf := zipEntryContent(t, zr, "OEBPS/content.opf")
	assert.NotContains(t, opf, "missing.jpg")
	index := zipEntryContent(t, zr, "OEBPS/index.xhtml")
	assert.NotContains(t, index, "<img")
}

func TestGoHTMLConverter_MalformedHTMLStillValid(t *testing.T) {
	malformed := `<html><body><p>unclosed paragraph<div>unclosed div` +
		`</span><br></body>`
	inPath := writeArticleFixture(t, malformed, nil)
	zr := convertToEPUBZip(
		t, inPath, ArticleMeta{Title: "Malformed", Authors: nil},
	)

	var buf bytes.Buffer
	err := kepubpkg.NewConverter().Convert(context.Background(), &buf, zr)
	require.NoError(t, err)
	assert.NotEmpty(t, buf.Bytes())
}

func TestGoHTMLConverter_KepubifyAcceptsRealisticArticle(t *testing.T) {
	png := []byte{0x89, 0x50, 0x4E, 0x47}
	body := `<html><head><title>My Article</title></head><body>` +
		`<h1>My Article</h1><p>Some text.</p>` +
		`<img src="img_0.png"></body></html>`
	inPath := writeArticleFixture(t, body, map[string][]byte{"img_0.png": png})
	zr := convertToEPUBZip(t, inPath, ArticleMeta{
		Title: "My Article", Authors: []string{"Jane Doe"},
	})

	var buf bytes.Buffer
	err := kepubpkg.NewConverter().Convert(context.Background(), &buf, zr)
	require.NoError(t, err)
	assert.NotEmpty(t, buf.Bytes())
}

func TestGoHTMLConverter_StripsScriptsAndHandlers(t *testing.T) {
	body := `<html><body>` +
		`<script>alert(1)</script>` +
		`<iframe src="https://evil.example"></iframe>` +
		`<video src="v.mp4"></video>` +
		`<audio src="a.mp3"></audio>` +
		`<form action="/x"><input></form>` +
		`<p onclick="evil()">text</p>` +
		`</body></html>`
	inPath := writeArticleFixture(t, body, nil)
	zr := convertToEPUBZip(
		t, inPath, ArticleMeta{Title: "Sanitize", Authors: nil},
	)

	index := zipEntryContent(t, zr, "OEBPS/index.xhtml")
	for _, tag := range []string{
		"<script", "<iframe", "<video", "<audio", "<form",
	} {
		assert.NotContains(t, index, tag)
	}
	assert.NotContains(t, index, "onclick")
	assert.Contains(t, index, "text")
}

func TestGoHTMLConverter_VoidElementsSelfClosed(t *testing.T) {
	body := `<html><body><p>a<br>b</p><hr><meta charset="utf-8"></body></html>`
	inPath := writeArticleFixture(t, body, nil)
	zr := convertToEPUBZip(t, inPath, ArticleMeta{Title: "Void", Authors: nil})

	index := zipEntryContent(t, zr, "OEBPS/index.xhtml")
	assert.Contains(t, index, "<br/>")
	assert.Contains(t, index, "<hr/>")
	assert.Regexp(t, `<meta[^>]*/>`, index)
}

func TestGoHTMLConverter_ContextCanceled(t *testing.T) {
	inPath := writeArticleFixture(t, "<html><body></body></html>", nil)
	outPath := filepath.Join(t.TempDir(), "out.epub")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	meta := ArticleMeta{Title: "Canceled", Authors: nil}
	err := goHTMLConverter(ctx, inPath, outPath, meta)
	require.Error(t, err)
	_, statErr := os.Stat(outPath)
	assert.True(t, os.IsNotExist(statErr))
}

func TestGoHTMLConverter_ReadInputError(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "out.epub")
	meta := ArticleMeta{Title: "", Authors: nil}
	err := goHTMLConverter(
		context.Background(), "/nonexistent/path/index.html", outPath, meta,
	)
	require.Error(t, err)
}

func TestGoHTMLConverter_CreateOutputError(t *testing.T) {
	inPath := writeArticleFixture(t, "<html><body></body></html>", nil)
	meta := ArticleMeta{Title: "", Authors: nil}
	err := goHTMLConverter(
		context.Background(), inPath, "/nonexistent/dir/out.epub", meta,
	)
	require.Error(t, err)
}

func TestResolveArticleImage(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "article")
	require.NoError(t, os.Mkdir(dir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "img_0.jpg"), []byte("data"), 0o600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "img_0.txt"), []byte("data"), 0o600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(parent, "secret.jpg"), []byte("secret"), 0o600,
	))

	tests := []struct {
		name string
		src  string
		want bool
	}{
		{"valid local file", "img_0.jpg", true},
		{"missing file", "missing.jpg", false},
		{"unsupported extension", "img_0.txt", false},
		{"path traversal", "../secret.jpg", false},
		{"absolute path", filepath.Join(parent, "secret.jpg"), false},
		{"remote url", "https://example.com/x.jpg", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := newTestNode(imgTag, []xhtml.Attribute{
				{Namespace: "", Key: srcAttr, Val: tt.src},
			})
			_, ok := resolveArticleImage(node, dir, 0)
			assert.Equal(t, tt.want, ok)
		})
	}
}

func TestNormalizeAttrs(t *testing.T) {
	node := newTestNode("input", []xhtml.Attribute{
		{Namespace: "", Key: "disabled", Val: ""},
		{Namespace: "", Key: "onclick", Val: "evil()"},
		{Namespace: "", Key: "onChange", Val: "evil()"},
		{Namespace: "", Key: "name", Val: "x"},
	})
	normalizeAttrs(node)

	assert.Equal(t, []xhtml.Attribute{
		{Namespace: "", Key: "disabled", Val: "disabled"},
		{Namespace: "", Key: "name", Val: "x"},
	}, node.Attr)
}

func TestRenderXHTMLDocument_NoHTMLElement(t *testing.T) {
	//nolint:exhaustruct // only Type matters: an empty document node
	root := &xhtml.Node{Type: xhtml.DocumentNode}
	_, err := renderXHTMLDocument(root)
	require.Error(t, err)
}

func TestSetAttrAndAttrValue(t *testing.T) {
	node := newTestNode("html", []xhtml.Attribute{
		{Namespace: "", Key: "xmlns", Val: "old"},
	})
	setAttr(node, "xmlns", "new")
	assert.Equal(t, "new", attrValue(node, "xmlns"))
	assert.Empty(t, attrValue(node, "missing"))

	node2 := newTestNode("html", nil)
	setAttr(node2, "lang", "en")
	assert.Equal(t, "en", attrValue(node2, "lang"))
}

func TestCopyImageEntry_MissingFileErrors(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	err := copyImageEntry(zw, t.TempDir(), epubImage{
		FileName:  "nope.jpg",
		MediaType: contentTypeJPEG,
		ID:        "item-img0",
	})
	require.Error(t, err)
}
