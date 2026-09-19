package services

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/feeds/internal/mocks"
)

const blogIndexHTML = `
<html>
<head><title>Example Blog</title></head>
<body>
<nav>
	<a href="/">Home</a>
	<a href="/tag/engineering">Engineering</a>
</nav>
<header>
	<a href="/signup">Create an account today</a>
</header>
<main>
	<article>
		<a href="/posts/first-post">Announcing our brand new feature today</a>
	</article>
	<article>
		<h2><a href="/posts/second-post">A deep dive into our infrastructure</a></h2>
	</article>
	<article>
		<a href="/posts/first-post">Announcing our brand new feature today (dup)</a>
	</article>
	<a href="https://other-site.example/posts/off-domain-post-with-a-long-title">
		Off domain post with a long title
	</a>
	<a href="/about">About</a>
	<a href="/posts/first-post#comments">Jump to comments</a>
</main>
<footer>
	<a href="/posts/footer-post-that-should-be-excluded">
		Footer post that should be excluded
	</a>
</footer>
</body>
</html>
`

func TestDiscoverPostLinks(t *testing.T) {
	links, err := discoverPostLinks("https://example.com/blog", []byte(blogIndexHTML))
	require.NoError(t, err)

	urls := make([]string, len(links))
	for i, l := range links {
		urls[i] = l.URL
	}

	assert.Equal(t, []string{
		"https://example.com/posts/first-post",
		"https://example.com/posts/second-post",
	}, urls)
	assert.Equal(t, "A deep dive into our infrastructure", links[1].Title)
}

func TestDiscoverPostLinksNoPlausibleLinks(t *testing.T) {
	html := `
	<html><body>
		<nav><a href="/posts/hidden-in-nav-with-a-long-title">Hidden nav post</a></nav>
		<a href="/">Home</a>
		<a href="/about">About</a>
		<a href="/tag/news">A tag page with a fairly long label</a>
	</body></html>
	`
	_, err := discoverPostLinks("https://example.com/blog", []byte(html))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoPostsFound))
}

func TestDiscoverPostLinksCapsAtMax(t *testing.T) {
	html := "<html><body><main>"
	for i := range maxDiscoveredLinks + 10 {
		n := strconv.Itoa(i)
		html += `<a href="/posts/post-` + n + `">Post number ` + n + ` has a long title</a>`
	}
	html += "</main></body></html>"

	links, err := discoverPostLinks("https://example.com/blog", []byte(html))
	require.NoError(t, err)
	assert.Len(t, links, maxDiscoveredLinks)
}

func TestDiscoverPostLinksBadPageURL(t *testing.T) {
	_, err := discoverPostLinks("http://[::1]:namedport", []byte(blogIndexHTML))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoPostsFound))
}

func TestCandidatePostURLRejections(t *testing.T) {
	html := `
	<html><body><main>
		<a>No href attribute at all, just a long title</a>
		<a href="">Empty href with an otherwise long title</a>
		<a href="#">Bare fragment link with a long title too</a>
		<a href="%zz">Malformed href escape with a long title</a>
		<a href="mailto:hello@example.com">Mailto link with a fairly long title</a>
		<a href="javascript:void(0)">Javascript link with a fairly long title</a>
		<a href="/posts/only-real-post-here">The only real post on this page</a>
	</main></body></html>
	`
	links, err := discoverPostLinks("https://example.com/blog", []byte(html))
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, "https://example.com/posts/only-real-post-here", links[0].URL)
}

func TestDiscoverPostLinksTimeOnlyCardStripsDateFromTitle(t *testing.T) {
	html := `
	<html><body><main>
		<a href="/research/some-report">
			<time>Jun 16, 2026</time>
			<span>Economic Research</span>
			<span>Agentic coding and persistent returns to expertise</span>
		</a>
	</main></body></html>
	`
	links, err := discoverPostLinks("https://example.com", []byte(html))
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(
		t,
		"Economic Research Agentic coding and persistent returns to expertise",
		links[0].Title,
	)
	assert.True(
		t,
		time.Date(2026, time.June, 16, 0, 0, 0, 0, time.UTC).
			Equal(links[0].PublishedAt),
	)
}

func TestDiscoverPostLinksHeadingCardPrefersHeadingOverDateAndExcerpt(t *testing.T) {
	html := `
	<html><body><main>
		<a href="/engineering/some-post">
			<h3>A postmortem of three recent issues</h3>
			<div>Sep 17, 2025</div>
		</a>
	</main></body></html>
	`
	links, err := discoverPostLinks("https://example.com", []byte(html))
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, "A postmortem of three recent issues", links[0].Title)
	assert.True(t, links[0].PublishedAt.IsZero())
}

func TestDiscoverPostLinksExcludesBareShortCategoryPills(t *testing.T) {
	html := `
	<html><body>
	<div class="filters">
		<a href="/research/frontier-red-team">Frontier red team</a>
		<a href="/research/societal-impacts">Societal impacts</a>
		<a href="/research/interpretability">Interpretability</a>
		<a href="/research/economic-research">Economic research</a>
	</div>
	<main>
		<a href="/research/some-actual-report">
			<time>Jun 16, 2026</time>
			<span>A real research report with a proper title</span>
		</a>
	</main>
	</body></html>
	`
	links, err := discoverPostLinks("https://example.com/research", []byte(html))
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, "https://example.com/research/some-actual-report", links[0].URL)
}

func TestDiscoverNextPageURLRelNextLink(t *testing.T) {
	html := `
	<html><head><link rel="next" href="/blog?page=2"></head>
	<body><main></main></body></html>
	`
	next, ok := discoverNextPageURL("https://example.com/blog", []byte(html))
	require.True(t, ok)
	assert.Equal(t, "https://example.com/blog?page=2", next)
}

func TestDiscoverNextPageURLAnchorText(t *testing.T) {
	html := `
	<html><body><main>
		<a href="/posts/some-post">Some post with a long title</a>
	</main>
	<nav class="pagination">
		<a href="/blog/page/2">Older Posts</a>
	</nav>
	</body></html>
	`
	next, ok := discoverNextPageURL("https://example.com/blog", []byte(html))
	require.True(t, ok)
	assert.Equal(t, "https://example.com/blog/page/2", next)
}

func TestDiscoverNextPageURLAriaLabel(t *testing.T) {
	html := `
	<html><body><main>
		<a href="/blog/page/2" aria-label="Next">›</a>
	</main></body></html>
	`
	next, ok := discoverNextPageURL("https://example.com/blog", []byte(html))
	require.True(t, ok)
	assert.Equal(t, "https://example.com/blog/page/2", next)
}

func TestDiscoverNextPageURLNoneFound(t *testing.T) {
	_, ok := discoverNextPageURL("https://example.com/blog", []byte(blogIndexHTML))
	assert.False(t, ok)
}

func TestDiscoverNextPageURLOffDomainRejected(t *testing.T) {
	html := `
	<html><body>
		<a rel="next" href="https://other.example/blog?page=2">Next</a>
	</body></html>
	`
	_, ok := discoverNextPageURL("https://example.com/blog", []byte(html))
	assert.False(t, ok)
}

func TestFetchPaginatedPostLinksFollowsNextPageAndDedupes(t *testing.T) {
	webFetch := mocks.NewMockWebFetchClient()
	webFetch.SetHTML("https://example.com/blog", `
		<html><body><main>
			<a href="/posts/page1-post">A post found on the very first page</a>
		</main>
		<a rel="next" href="/blog?page=2">Next</a>
		</body></html>
	`)
	webFetch.SetHTML("https://example.com/blog?page=2", `
		<html><body><main>
			<a href="/posts/page2-post">A post found on the second page</a>
			<a href="/posts/page1-post">A post found on the very first page (dup)</a>
		</main>
		</body></html>
	`)

	s := NewFeedService(slog.Default(), nil, nil, webFetch, "", nil, nil, "")
	links, err := s.fetchPaginatedPostLinks(
		context.Background(), "https://example.com/blog",
		webFetch.Responses["https://example.com/blog"].Body,
	)
	require.NoError(t, err)

	urls := make([]string, len(links))
	for i, l := range links {
		urls[i] = l.URL
	}
	assert.Equal(t, []string{
		"https://example.com/posts/page1-post",
		"https://example.com/posts/page2-post",
	}, urls)
	assert.Equal(t, []string{"https://example.com/blog?page=2"}, webFetch.Calls)
}

func TestFetchPaginatedPostLinksStopsWhenNextPageHasNoPosts(t *testing.T) {
	webFetch := mocks.NewMockWebFetchClient()
	firstPage := []byte(`
		<html><body><main>
			<a href="/posts/only-post">The only post on this whole site</a>
		</main>
		<a rel="next" href="/blog?page=2">Next</a>
		</body></html>
	`)
	webFetch.SetBody("https://example.com/blog?page=2", "text/html", []byte(`
		<html><body><nav><a href="/">Home</a></nav></body></html>
	`))

	s := NewFeedService(slog.Default(), nil, nil, webFetch, "", nil, nil, "")
	links, err := s.fetchPaginatedPostLinks(
		context.Background(), "https://example.com/blog", firstPage,
	)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, "https://example.com/posts/only-post", links[0].URL)
}

func TestFetchPaginatedPostLinksCapsAtMaxScrapePages(t *testing.T) {
	webFetch := mocks.NewMockWebFetchClient()
	page := func(n int) string {
		return "https://example.com/blog?page=" + strconv.Itoa(n)
	}
	firstPage := []byte(`
		<html><body><main>
			<a href="/posts/page-1-post">A post found on page number one</a>
		</main>
		<a rel="next" href="/blog?page=2">Next</a>
		</body></html>
	`)
	for n := 2; n <= 5; n++ {
		next := n + 1
		webFetch.SetHTML(page(n), `
			<html><body><main>
				<a href="/posts/page-`+strconv.Itoa(
			n,
		)+`-post">A post discovered on page number `+strconv.Itoa(
			n,
		)+`</a>
			</main>
			<a rel="next" href="/blog?page=`+strconv.Itoa(next)+`">Next</a>
			</body></html>
		`)
	}

	s := NewFeedService(slog.Default(), nil, nil, webFetch, "", nil, nil, "")
	links, err := s.fetchPaginatedPostLinks(
		context.Background(), "https://example.com/blog", firstPage,
	)
	require.NoError(t, err)
	assert.Len(t, links, maxScrapePages)
	assert.Equal(t, []string{page(2), page(3)}, webFetch.Calls)
}

func TestPageTitle(t *testing.T) {
	title := pageTitle([]byte(blogIndexHTML))
	assert.Equal(t, "Example Blog", title)
}

func TestPageTitleMissing(t *testing.T) {
	title := pageTitle([]byte("<html><body>no title here</body></html>"))
	assert.Equal(t, "", title)
}
