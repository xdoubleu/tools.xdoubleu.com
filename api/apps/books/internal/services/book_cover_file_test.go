//nolint:testpackage // testing unexported service helpers
package services

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/pkg/objectstore"
)

const epubCoverJPEG = "\xff\xd8\xff\xe0cover"

func coverZip(t *testing.T, opfBody string, files map[string]string) *zip.Reader {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	entries := map[string]string{
		"META-INF/container.xml": `<container><rootfiles>` +
			`<rootfile full-path="OEBPS/content.opf" media-type="` + opfMediaType +
			`"/></rootfiles></container>`,
		"OEBPS/content.opf": `<package xmlns="http://www.idpf.org/2007/opf">` +
			opfBody + `</package>`,
	}
	for name, body := range files {
		entries[name] = body
	}
	for name, body := range entries {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	return zr
}

func TestEPUBCover(t *testing.T) {
	tests := []struct {
		name  string
		opf   string
		files map[string]string
		// want is empty when no cover is found.
		want string
	}{
		{
			name: "epub3 cover-image",
			opf: `<metadata/><manifest>` +
				`<item id="x" href="text.xhtml" media-type="application/xhtml+xml"/>` +
				`<item id="c" href="img/cover%20art.jpg" media-type="image/jpeg"` +
				` properties="cover-image"/></manifest>`,
			files: map[string]string{"OEBPS/img/cover art.jpg": epubCoverJPEG},
			want:  epubCoverJPEG,
		},
		{
			name: "epub2 meta cover",
			opf: `<metadata><meta name="cover" content="cov"/></metadata>` +
				`<manifest><item id="cov" href="../cover.jpg" media-type="image/jpeg"/>` +
				`</manifest>`,
			files: map[string]string{"cover.jpg": epubCoverJPEG},
			want:  epubCoverJPEG,
		},
		{
			name: "meta naming a non-image",
			opf: `<metadata><meta name="cover" content="cov"/></metadata>` +
				`<manifest><item id="cov" href="cover.xhtml"` +
				` media-type="application/xhtml+xml"/></manifest>`,
			files: map[string]string{"OEBPS/cover.xhtml": "<html/>"},
			want:  "",
		},
		{
			name:  "no cover",
			opf:   `<metadata/><manifest/>`,
			files: nil,
			want:  "",
		},
		{
			name: "cover entry absent",
			opf: `<metadata/><manifest><item id="c" href="gone.jpg"` +
				` media-type="image/jpeg" properties="cover-image"/></manifest>`,
			files: nil,
			want:  "",
		},
		{
			name: "cover entry not an image",
			opf: `<metadata/><manifest><item id="c" href="c.jpg"` +
				` media-type="image/jpeg" properties="cover-image"/></manifest>`,
			files: map[string]string{"OEBPS/c.jpg": "<html>not an image</html>"},
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := epubCover(coverZip(t, tt.opf, tt.files))
			if tt.want == "" {
				assert.ErrorIs(t, err, ErrCoverNotFound)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(data))
		})
	}
}

func TestEPUBCover_NoContainer(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, zip.NewWriter(&buf).Close())
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)

	_, err = epubCover(zr)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrCoverNotFound)
}

func TestOpenStoredZip_MissingObject(t *testing.T) {
	svc := &BookService{objectStore: objectstore.NewFake()} //nolint:exhaustruct // partial
	_, err := svc.openStoredZip(t.Context(), "books/missing.epub")
	assert.Error(t, err)
}
