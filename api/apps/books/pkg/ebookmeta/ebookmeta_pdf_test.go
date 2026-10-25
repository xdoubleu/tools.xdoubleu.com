package ebookmeta_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/books/pkg/ebookmeta"
)

// The pdf library panics while lazily resolving a malformed object; uploads
// are untrusted, so that must surface as an error.
func TestExtract_PDF_MalformedInfoObjectReturnsError(t *testing.T) {
	t.Parallel()

	// The xref entry for the Info object (4 0 R) now points at "9 0 obj".
	data := bytes.Replace(
		buildTestPDF("Title", "Author"), []byte("4 0 obj"), []byte("9 0 obj"), 1,
	)

	got, err := ebookmeta.Extract(
		ebookmeta.FormatPDF, bytes.NewReader(data), int64(len(data)),
	)
	assert.ErrorContains(t, err, "ebookmeta: read pdf metadata")
	assert.Equal(t, ebookmeta.Metadata{}, got) //nolint:exhaustruct // zero value
}
