//nolint:testpackage // testing unexported service helpers
package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsHTMLContentType(t *testing.T) {
	assert.True(t, isHTMLContentType("text/html"))
	assert.True(t, isHTMLContentType("application/xhtml+xml"))
	assert.True(t, isHTMLContentType("")) // no header: assume HTML
	assert.False(t, isHTMLContentType("application/pdf"))
	assert.False(t, isHTMLContentType("image/png"))
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
