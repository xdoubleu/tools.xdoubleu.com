package sniff_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/books/pkg/sniff"
)

func TestContentType(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	assert.Equal(t, "image/png", sniff.ContentType(png))
	assert.Equal(t, "text/plain; charset=utf-8", sniff.ContentType([]byte("hello")))
}
