//nolint:testpackage // testing unexported span-map helpers
package services

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/mocks"
	"tools.xdoubleu.com/apps/books/internal/models"
)

// chapterZips returns ChapterEPUB and its real kepubify conversion.
func chapterZips(t *testing.T) (*zip.Reader, *zip.Reader) {
	t.Helper()
	orig := mocks.ChapterEPUB("Spans", "Author")
	kepub, err := newKepubifyConverter().Convert(context.Background(), orig)
	require.NoError(t, err)
	return openZip(t, orig), openZip(t, kepub)
}

func openZip(t *testing.T, data []byte) *zip.Reader {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	return zr
}

func docTextOf(t *testing.T, zr *zip.Reader, name string) docText {
	t.Helper()
	f, err := zr.Open(name)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	dt, err := parseBodyText(f)
	require.NoError(t, err)
	return dt
}

func str(units []uint16) string { return string(utf16.Decode(units)) }

// TestKepubify_BodyTextMatchesUpToTrailer: for ordinary content kepubify keeps
// the body text; its HTML5 parser only moves the whitespace after </body> in.
func TestKepubify_BodyTextMatchesUpToTrailer(t *testing.T) {
	orig, kepub := chapterZips(t)
	for _, href := range []string{mocks.ChapterOneHref, mocks.ChapterTwoHref} {
		o := str(docTextOf(t, orig, href).units)
		k := docTextOf(t, kepub, href)
		ks := str(k.units)
		require.True(t, strings.HasPrefix(ks, o), "%s: %q vs %q", href, o, ks)
		assert.Empty(t, strings.TrimSpace(ks[len(o):]), href)
		require.NotEmpty(t, k.spans, href)
		for _, s := range k.spans {
			assert.LessOrEqual(t, s.end, len(utf16.Encode([]rune(o))),
				"%s: span %s in the moved-in trailer", href, s.id)
		}
	}
	assert.Equal(t, mocks.ChapterTwoText,
		str(docTextOf(t, orig, mocks.ChapterTwoHref).units))
}

// TestKepubify_ChangesTextOfHTML5OnlyConstructs records where kepubify's text
// differs, which is why buildSpanMap aligns rather than assumes equality.
func TestKepubify_ChangesTextOfHTML5OnlyConstructs(t *testing.T) {
	orig, kepub := chapterZips(t)
	o := str(docTextOf(t, orig, mocks.ChapterThreeHref).units)
	k := str(docTextOf(t, kepub, mocks.ChapterThreeHref).units)
	assert.NotEqual(t, o, k)
	assert.Contains(t, o, "\ncode line")
	assert.NotContains(t, k, "\n\ncode line", "pre loses its leading newline")
	assert.Contains(t, o, "Text raw  more.")
	assert.NotContains(t, k, "Text raw  more.", "CDATA becomes a comment")
	assert.Contains(t, o, "�")
	assert.NotContains(t, k, "�", "kepubify strips U+FFFD")
}

// TestBuildSpanMap_EverySpanStartsAtItsTextInOriginal: every span whose text
// kepubify kept lands on that text in the original's body.
func TestBuildSpanMap_EverySpanStartsAtItsTextInOriginal(t *testing.T) {
	orig, kepub := chapterZips(t)
	m, err := buildSpanMap(context.Background(), kepub, orig)
	require.NoError(t, err)

	for _, href := range []string{
		mocks.ChapterOneHref, mocks.ChapterTwoHref, mocks.ChapterThreeHref,
	} {
		ou := docTextOf(t, orig, href).units
		kt := docTextOf(t, kepub, href)
		d := m.doc(href)
		require.NotNil(t, d, href)
		require.Len(t, d.spans, len(kt.spans), href)

		altered := 0
		for i, ks := range kt.spans {
			got := d.spans[i]
			require.Equal(t, ks.id, got.id)
			text := kt.units[ks.start:ks.end]
			if !strings.Contains(str(ou), str(text)) {
				altered++
				continue
			}
			end := int(got.start) + len(text)
			require.LessOrEqual(t, end, len(ou), "%s %s", href, ks.id)
			assert.Equal(t, str(text), str(ou[got.start:end]), "%s %s", href, ks.id)
		}
		if href == mocks.ChapterThreeHref {
			assert.Equal(t, 2, altered, "the noscript and U+FFFD spans")
		} else {
			assert.Zero(t, altered, href)
		}
	}
}

func TestSpanMap_RoundTripsEverySpan(t *testing.T) {
	orig, kepub := chapterZips(t)
	m, err := buildSpanMap(context.Background(), kepub, orig)
	require.NoError(t, err)
	require.Len(t, m.docs, 3)

	for _, d := range m.docs {
		for i, s := range d.spans {
			pos := m.position(models.KoboLocation{
				Source: d.href, Type: koboSpanType, Value: s.id.String(),
			})
			require.NotNil(t, pos, s.id.String())
			assert.Equal(t, d.href, pos.Href)
			assert.Equal(t, int(s.start), pos.Offset)

			loc := m.location(*pos)
			require.NotNil(t, loc)
			first := i
			for first > 0 && d.spans[first-1].start == s.start {
				first--
			}
			assert.Equal(t, models.KoboLocation{
				Source: d.href, Type: koboSpanType, Value: d.spans[first].id.String(),
			}, *loc)
		}
	}
}

func TestSpanMap_ChapterTwoOffsets(t *testing.T) {
	orig, kepub := chapterZips(t)
	m, err := buildSpanMap(context.Background(), kepub, orig)
	require.NoError(t, err)

	second := strings.Index(mocks.ChapterTwoText, "It continues.")
	pos := m.position(models.KoboLocation{
		Source: mocks.ChapterTwoHref, Type: koboSpanType, Value: "kobo.1.2",
	})
	require.NotNil(t, pos)
	assert.Equal(t, models.ReadingPosition{
		Href: mocks.ChapterTwoHref, Offset: second, Page: 0,
	}, *pos)

	for offset, want := range map[int]string{
		0:                                  "kobo.1.1",
		second - 1:                         "kobo.1.1",
		second:                             "kobo.1.2",
		second + 5:                         "kobo.1.2",
		len(mocks.ChapterTwoText) - 1:      "kobo.2.1",
		len(mocks.ChapterTwoText) + 100000: "kobo.2.1",
	} {
		loc := m.location(models.ReadingPosition{
			Href: mocks.ChapterTwoHref, Offset: offset, Page: 0,
		})
		require.NotNil(t, loc, offset)
		assert.Equal(t, want, loc.Value, "offset %d", offset)
	}
}

func TestSpanMap_Misses(t *testing.T) {
	orig, kepub := chapterZips(t)
	m, err := buildSpanMap(context.Background(), kepub, orig)
	require.NoError(t, err)

	assert.Nil(t, m.position(models.KoboLocation{
		Source: mocks.ChapterTwoHref, Type: koboSpanType, Value: "kobo.99.1",
	}))
	assert.Nil(t, m.position(models.KoboLocation{
		Source: mocks.ChapterTwoHref, Type: koboSpanType, Value: "not-a-span",
	}))
	assert.Nil(t, m.position(models.KoboLocation{
		Source: "OEBPS/Text/missing.xhtml", Type: koboSpanType, Value: "kobo.1.1",
	}))
	assert.Nil(t, m.location(models.ReadingPosition{
		Href: "OEBPS/Text/missing.xhtml", Offset: 0, Page: 0,
	}))
	assert.Nil(t, m.location(models.ReadingPosition{Href: "", Offset: 0, Page: 3}))
}

func TestSpanMap_DocMatchesSourceVariants(t *testing.T) {
	m := &spanMap{docs: []docSpans{
		{href: "OEBPS/Text/ch1.xhtml", spans: nil},
		{href: "OEBPS/a/same.xhtml", spans: nil},
		{href: "OEBPS/b/same.xhtml", spans: nil},
	}}
	for source, want := range map[string]string{
		"OEBPS/Text/ch1.xhtml":                   "OEBPS/Text/ch1.xhtml",
		"oebps/text/CH1.xhtml":                   "OEBPS/Text/ch1.xhtml",
		"Text/ch1.xhtml":                         "OEBPS/Text/ch1.xhtml",
		"ch1.xhtml":                              "OEBPS/Text/ch1.xhtml",
		"/mnt/onboard/book/OEBPS/Text/ch1.xhtml": "OEBPS/Text/ch1.xhtml",
		"OEBPS/a/same.xhtml":                     "OEBPS/a/same.xhtml",
		"same.xhtml":                             "",
		"h1.xhtml":                               "",
		"":                                       "",
		"OEBPS/Text/ch2.xhtml":                   "",
		"file:///mnt/onboard/OEBPS/Text/ch1.xhtml": "OEBPS/Text/ch1.xhtml",
	} {
		d := m.doc(source)
		if want == "" {
			assert.Nil(t, d, source)
			continue
		}
		require.NotNil(t, d, source)
		assert.Equal(t, want, d.href, source)
	}
}

func TestParseBodyText_CountsLikeTextContent(t *testing.T) {
	doc := `<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Not body</title></head>` +
		`<body><p>a&amp;b&nbsp;<![CDATA[<c>]]><!-- no --></p>` +
		`<script>x&lt;y</script><style>s</style>` +
		`<p><span class="koboSpan" id="kobo.3.4">` + "\U0001F600" + `z</span></p>` +
		`<span id="other">o</span></body><p>after body</p></html>`
	dt, err := parseBodyText(strings.NewReader(doc))
	require.NoError(t, err)
	assert.Equal(t, "a&b <c>x<ys\U0001F600zo", str(dt.units))
	require.Len(t, dt.spans, 1)
	assert.Equal(t, "kobo.3.4", dt.spans[0].id.String())
	assert.Equal(t, 11, dt.spans[0].start)
	assert.Equal(t, 14, dt.spans[0].end, "the emoji counts two UTF-16 units")
}

func TestParseBodyText_DeclaredCharset(t *testing.T) {
	doc := []byte("<?xml version=\"1.0\" encoding=\"iso-8859-1\"?>" +
		"<html><body><p>caf\xe9</p></body></html>")
	dt, err := parseBodyText(bytes.NewReader(doc))
	require.NoError(t, err)
	assert.Equal(t, "café", str(dt.units))
}

func TestParseBodyText_Malformed(t *testing.T) {
	_, err := parseBodyText(strings.NewReader("<html><body><p>a\xff</p></body></html>"))
	assert.Error(t, err)
}

func TestParseSpanID(t *testing.T) {
	id, ok := parseSpanID("kobo.12.3")
	require.True(t, ok)
	assert.Equal(t, "kobo.12.3", id.String())
	for _, bad := range []string{"", "kobo.1", "kobo.a.1", "kobo.1.b", "span.1.2",
		"kobo.1.2.3", "kobo.-1.2", "kobo.99999999999.1"} {
		_, ok = parseSpanID(bad)
		assert.False(t, ok, bad)
	}
}

func TestBuildSpanMap_BadArchives(t *testing.T) {
	orig, kepub := chapterZips(t)
	empty := openZip(t, func() []byte {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		_ = zw.Close()
		return buf.Bytes()
	}())
	_, err := buildSpanMap(context.Background(), empty, orig)
	require.Error(t, err, "no container.xml")

	// A KEPUB doc the original lacks (e.g. kepubify's dummy title page) is
	// skipped, not an error.
	noCh3 := stripEntry(t, orig, mocks.ChapterThreeHref)
	m, err := buildSpanMap(context.Background(), kepub, noCh3)
	require.NoError(t, err)
	assert.Nil(t, m.doc(mocks.ChapterThreeHref))
	assert.NotNil(t, m.doc(mocks.ChapterTwoHref))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = buildSpanMap(ctx, kepub, orig)
	assert.ErrorIs(t, err, context.Canceled)
}

func stripEntry(t *testing.T, zr *zip.Reader, name string) *zip.Reader {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		if f.Name == name {
			continue
		}
		require.NoError(t, zw.Copy(f))
	}
	require.NoError(t, zw.Close())
	return openZip(t, buf.Bytes())
}
