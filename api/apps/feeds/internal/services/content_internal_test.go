package services

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const articleParagraph = `<p>Lorem ipsum dolor sit amet, consectetur adipiscing
elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut
enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut
aliquip ex ea commodo consequat.</p>`

func articlePage(head string) []byte {
	return []byte(`<html><head><meta charset="utf-8">` + head + `</head><body>` +
		`<article><h1>Heading</h1>` + articleParagraph + articleParagraph +
		`</article></body></html>`)
}

func TestExtractReadable(t *testing.T) {
	art, err := extractReadable(
		"https://blog.example.com/post", articlePage("<title>My Post</title>"),
	)
	require.NoError(t, err)
	assert.Equal(t, "My Post", art.Title)
	assert.Contains(t, art.HTML, "<p>Lorem ipsum dolor sit amet")
}

func TestExtractReadable_TitleFallsBackToURL(t *testing.T) {
	art, err := extractReadable("https://blog.example.com/post", articlePage(""))
	require.NoError(t, err)
	assert.Equal(t, "https://blog.example.com/post", art.Title)
}

// Postgres text rejects invalid UTF-8; readability's charset decoding must
// never let it through.
func TestExtractReadable_InvalidUTF8YieldsValidText(t *testing.T) {
	art, err := extractReadable(
		"https://blog.example.com/post", articlePage("<title>Bad \xff title</title>"),
	)
	require.NoError(t, err)
	assert.True(t, utf8.ValidString(art.Title))
	assert.True(t, utf8.ValidString(art.HTML))
}

func TestExtractReadable_NoContent(t *testing.T) {
	_, err := extractReadable(
		"https://example.com/empty", []byte("<html><body></body></html>"),
	)
	require.ErrorIs(t, err, ErrNoReadableContent)
}
