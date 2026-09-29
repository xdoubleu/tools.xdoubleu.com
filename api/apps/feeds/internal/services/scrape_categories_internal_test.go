package services

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// claudeBlogFixture is a captured claude.com/blog index (Webflow + Finsweet):
// each card's fs-list-field="category" label sits beside the link overlay.
//
//go:embed testdata/claude-blog-index.html.gz
var claudeBlogFixture []byte

func linkCategories(links []discoveredLink) map[string][]string {
	out := make(map[string][]string, len(links))
	for _, link := range links {
		out[link.URL] = link.Categories
	}
	return out
}

func TestDiscoverPostLinksClaudeBlogCardCategories(t *testing.T) {
	zr, err := gzip.NewReader(bytes.NewReader(claudeBlogFixture))
	require.NoError(t, err)
	body, err := io.ReadAll(zr)
	require.NoError(t, err)

	links, err := discoverPostLinks("https://claude.com/blog", body)
	require.NoError(t, err)

	categories := linkCategories(links)
	const base = "https://claude.com/blog/"
	assert.Equal(t, []string{"Product announcements"},
		categories[base+"claude-tag-now-supports-personal-connectors-in-channels"])
	assert.Equal(t, []string{"Enterprise AI"},
		categories[base+"how-to-prepare-for-ai-driven-code-modernization-projects"])
	assert.Equal(
		t,
		[]string{"Claude Code"},
		categories[base+"claude-opus-5-5-built-for-coding-sessions-that-use-more-context"],
	)
}

const taggedIndexHTML = `<html><body>
<nav><a href="/posts/nav-post-with-a-long-enough-title">Nav post</a>
	<span class="tag">Nav chrome</span></nav>
<main><ul>
	<li><article>
		<h2><a href="/posts/first-tagged-post">The first tagged post of the blog</a></h2>
		<div class="post-tags">
			<a rel="tag" href="/topics/go"> Go </a>
			<a rel="tag" href="/topics/db">Databases</a>
			<a rel="tag" href="/topics/go-again">go</a>
		</div>
	</article></li>
	<li><article>
		<h2><a href="/posts/second-tagged-post">The second tagged post of the blog</a></h2>
		<span class="card_category-label">Engineering</span>
		<span class="category">This label is far too long to be a category chip</span>
		<span class="tagline">Not a tag: tagline is its own word</span>
	</article></li>
	<li><article>
		<h2><a href="/posts/untagged-post">An untagged post of the blog here</a></h2>
	</article></li>
</ul></main>
</body></html>`

func TestDiscoverPostLinksCardCategories(t *testing.T) {
	links, err := discoverPostLinks("https://example.com/blog", []byte(taggedIndexHTML))
	require.NoError(t, err)

	assert.Equal(t, map[string][]string{
		"https://example.com/posts/first-tagged-post":  {"Go", "Databases"},
		"https://example.com/posts/second-tagged-post": {"Engineering"},
		"https://example.com/posts/untagged-post":      nil,
	}, linkCategories(links))
}

func TestNormalizeCategories(t *testing.T) {
	assert.Equal(t,
		[]string{"Product announcements", "AI"},
		normalizeCategories([]string{
			"  Product   announcements ", "", "   ", "AI", "product announcements", "ai",
		}),
	)
	assert.Nil(t, normalizeCategories(nil))
}
