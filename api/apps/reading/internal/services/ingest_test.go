//nolint:testpackage // testing unexported service helpers
package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/reading/pkg/webfetch"
)

func TestCanonicalURL(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{
			"lowercases scheme and host",
			"HTTPS://Example.COM/Article",
			"https://example.com/Article",
			false,
		},
		{
			"strips fragment",
			"https://example.com/a#section-2",
			"https://example.com/a",
			false,
		},
		{
			"strips utm params, keeps others",
			"https://example.com/a?utm_source=x&id=7&UTM_campaign=y",
			"https://example.com/a?id=7",
			false,
		},
		{"rejects ftp", "ftp://example.com/a", "", true},
		{"rejects garbage", "not a url", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := canonicalURL(tt.in)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrUnsupportedURL)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsHTMLContentType(t *testing.T) {
	assert.True(t, isHTMLContentType("text/html"))
	assert.True(t, isHTMLContentType("application/xhtml+xml"))
	assert.True(t, isHTMLContentType("")) // no header: assume HTML
	assert.False(t, isHTMLContentType("application/pdf"))
	assert.False(t, isHTMLContentType("image/png"))
}

func TestTitleFromPDF_FallsBackToURLSegment(t *testing.T) {
	// Not a real PDF, so metadata extraction fails and the URL wins.
	title := titleFromPDF(
		[]byte("junk"), "https://example.com/papers/attention.pdf",
	)
	assert.Equal(t, "attention", title)
}

func TestExtractReadable(t *testing.T) {
	page := `<html><head><title>My Post — Blog</title>
<meta name="author" content="Jane Doe"></head><body>
<article><h1>My Post</h1>` +
		"<p>" + loremParagraph + "</p><p>" + loremParagraph + "</p>" +
		`</article></body></html>`

	art, err := extractReadable("https://blog.example.com/my-post", []byte(page))
	require.NoError(t, err)
	assert.NotEmpty(t, art.Title)
	assert.Contains(t, art.HTML, "<p>")
}

// loremParagraph gives readability enough text to accept the page.
const loremParagraph = `Lorem ipsum dolor sit amet, consectetur adipiscing
elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut
enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut
aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in
voluptate velit esse cillum dolore eu fugiat nulla pariatur.`

func TestExtractReadable_NoContent(t *testing.T) {
	_, err := extractReadable(
		"https://example.com/empty", []byte("<html><body></body></html>"),
	)
	assert.ErrorIs(t, err, ErrNoReadableContent)
}

func TestArticleHTMLDocument_EscapesMetadata(t *testing.T) {
	art := &extractedArticle{ //nolint:exhaustruct // partial fixture is intentional
		Title:  `Ben & Jerry's <script>`,
		Byline: "A <b>uthor",
	}
	doc := articleHTMLDocument(art, "<p>body</p>")

	assert.Contains(t, doc, "Ben &amp; Jerry&#39;s &lt;script&gt;")
	assert.Contains(t, doc, "A &lt;b&gt;uthor")
	assert.Contains(t, doc, "<p>body</p>")
	assert.NotContains(t, doc, "<script>")
}

func TestBuildArticleEPUB_UsesInjectedConverter(t *testing.T) {
	var gotMeta ArticleMeta
	fakeConvert := func(
		_ context.Context, inPath, outPath string, meta ArticleMeta,
	) error {
		gotMeta = meta
		data, err := os.ReadFile(inPath)
		if err != nil {
			return err
		}
		assert.Contains(t, string(data), "<h1>Post</h1>")
		// The "EPUB" is just bytes here — storage does not re-validate.
		return os.WriteFile(outPath, []byte("epub-bytes"), 0o600)
	}

	s := &IngestService{ //nolint:exhaustruct // only fields the build path uses
		logger:      logging.NewNopLogger(),
		webFetch:    &stubFetcher{},
		htmlConvert: fakeConvert,
	}

	art := &extractedArticle{ //nolint:exhaustruct // partial fixture is intentional
		Title:  "Post",
		Byline: "Jane",
		HTML:   "<p>hello</p>",
	}
	epub, err := s.buildArticleEPUB(
		context.Background(), art, "https://blog.example.com/post",
	)
	require.NoError(t, err)
	assert.Equal(t, "epub-bytes", string(epub))
	assert.Equal(t, "Post", gotMeta.Title)
	assert.Equal(t, []string{"Jane"}, gotMeta.Authors)
}

// stubFetcher fails every fetch — good enough for image-less articles.
type stubFetcher struct{}

func (s *stubFetcher) Get(
	_ context.Context, _ string, _ webfetch.Options,
) (*webfetch.Result, error) {
	return nil, webfetch.ErrStatus
}

func TestLocalizeImages(t *testing.T) {
	dir := t.TempDir()
	fetch := &mapFetcher{responses: map[string]*webfetch.Result{
		"https://cdn.example.com/pic.png": {

			Body:        []byte("png-bytes"),
			ContentType: "image/png",
		},
	}}
	s := &IngestService{ //nolint:exhaustruct // only fields the image path uses
		logger:   logging.NewNopLogger(),
		webFetch: fetch,
	}

	html := `<div><img src="/pic.png" srcset="a 1x, b 2x">` +
		`<img src="https://dead.example.com/x.jpg"><p>text</p></div>`
	out := s.localizeImages(
		context.Background(), dir, html, "https://cdn.example.com/post",
	)

	// Downloaded image rewritten to a local file, srcset dropped.
	assert.Contains(t, out, `src="img_0.png"`)
	assert.NotContains(t, out, "srcset")
	// Failed image stripped entirely; other content preserved.
	assert.NotContains(t, out, "dead.example.com")
	assert.Contains(t, out, "<p>text</p>")

	written, err := os.ReadFile(filepath.Join(dir, "img_0.png"))
	require.NoError(t, err)
	assert.Equal(t, "png-bytes", string(written))
}

type mapFetcher struct {
	responses map[string]*webfetch.Result
}

func (m *mapFetcher) Get(
	_ context.Context, url string, _ webfetch.Options,
) (*webfetch.Result, error) {
	if res, ok := m.responses[url]; ok {
		return res, nil
	}
	return nil, webfetch.ErrStatus
}
