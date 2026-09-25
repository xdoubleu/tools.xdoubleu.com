package services

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
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

func TestDiscoverPostLinksExcludesLocaleSwitcherLink(t *testing.T) {
	// A same-page locale switcher outside nav chrome, with title-length text.
	html := `
	<html><body>
	<div class="language-switcher">
		<a href="/be/fr/blog/engineering/?id=222">French, Français (France)</a>
		<a href="/be/de/blog/engineering/?id=160">German, Deutsch (Germany)</a>
	</div>
	<main>
		<a href="/be/en/blog/junit-migration/">
			How Uber Executed A JUnit Migration at Massive Scale
		</a>
	</main>
	</body></html>
	`
	links, err := discoverPostLinks(
		"https://www.uber.com/be/en/blog/engineering", []byte(html),
	)
	require.NoError(t, err)

	urls := make([]string, len(links))
	for i, l := range links {
		urls[i] = l.URL
	}
	assert.Equal(t, []string{
		"https://www.uber.com/be/en/blog/junit-migration/",
	}, urls)
}

func TestDiscoverPostLinksExcludesLocaleSwitcherLinkRegardlessOfQueryString(
	t *testing.T,
) {
	// The switcher href with volatile query strings must be rejected every
	// time; canonicalURL only strips utm_* so dedup can't catch it.
	queries := []string{
		"",
		"?id=222",
		"?id=160",
		"?countryiso2=us%2525255cu0022",
		"?id=0XB7q&userId=_msv+_31_7734666612221001011",
	}
	for _, q := range queries {
		html := `<html><body><main>` +
			`<a href="/be/fr/blog/engineering/` + q + `">French, Français (France)</a>` +
			`</main></body></html>`
		_, err := discoverPostLinks(
			"https://www.uber.com/be/en/blog/engineering", []byte(html),
		)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNoPostsFound))
	}
}

func TestIsLocaleAlternateRequiresSameSegmentCountAndSingleLocaleDiff(t *testing.T) {
	base, err := url.Parse("https://www.uber.com/be/en/blog/engineering")
	require.NoError(t, err)

	tests := []struct {
		name     string
		resolved string
		want     bool
	}{
		{
			name:     "single locale segment swap",
			resolved: "https://www.uber.com/be/fr/blog/engineering",
			want:     true,
		},
		{
			name:     "region-qualified locale segment swap",
			resolved: "https://www.uber.com/be/en-US/blog/engineering",
			want:     true,
		},
		{
			name:     "different segment count is not an alternate",
			resolved: "https://www.uber.com/be/en/blog/engineering/page/2",
			want:     false,
		},
		{
			name:     "differing non-locale-shaped segment is a real post",
			resolved: "https://www.uber.com/be/en/blog/junit-migration",
			want:     false,
		},
		{
			name:     "more than one differing segment is not an alternate",
			resolved: "https://www.uber.com/xx/fr/blog/engineering",
			want:     false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolved, parseErr := url.Parse(tc.resolved)
			require.NoError(t, parseErr)
			assert.Equal(t, tc.want, isLocaleAlternate(resolved, base))
		})
	}
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

// The site-wide locale switcher on page 2 must be judged against the first
// page's URL, since page 2's URL has extra segments.
func TestFetchPaginatedPostLinksFiltersLocaleSwitcherOnLaterPages(t *testing.T) {
	webFetch := mocks.NewMockWebFetchClient()
	switcher := `
		<a href="/be/fr/blog/engineering/">French, Français (France)</a>
		<a href="/be/en-US/blog/engineering/">English, United States</a>
	`
	webFetch.SetHTML("https://www.uber.com/be/en/blog/engineering", `
		<html><body><main>
			<a href="/be/en/blog/engineering/page-one-post">A post found on the first page</a>
		</main>
		`+switcher+`
		<a rel="next" href="/be/en/blog/engineering/page/2">Next</a>
		</body></html>
	`)
	webFetch.SetHTML("https://www.uber.com/be/en/blog/engineering/page/2", `
		<html><body><main>
			<a href="/be/en/blog/engineering/junit-migration-at-scale">
				How Uber Executed A JUnit Migration at Massive Scale
			</a>
		</main>
		`+switcher+`
		</body></html>
	`)

	s := NewFeedService(slog.Default(), nil, nil, webFetch, "", nil, nil, "")
	links, err := s.fetchPaginatedPostLinks(
		context.Background(),
		"https://www.uber.com/be/en/blog/engineering",
		webFetch.Responses["https://www.uber.com/be/en/blog/engineering"].Body,
	)
	require.NoError(t, err)

	urls := make([]string, len(links))
	for i, l := range links {
		urls[i] = l.URL
	}
	assert.Equal(t, []string{
		"https://www.uber.com/be/en/blog/engineering/page-one-post",
		"https://www.uber.com/be/en/blog/engineering/junit-migration-at-scale",
	}, urls)
}

// The walk exhausts pagination (no page cap) and ends when a page adds
// nothing new, simulated here by the last page repeating a seen post.
func TestFetchPaginatedPostLinksWalksUntilNoNewLinks(t *testing.T) {
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
	const lastPage = 5
	for n := 2; n <= lastPage; n++ {
		next := n + 1
		hrefs := `<a href="/posts/page-` + strconv.Itoa(n) +
			`-post">A post discovered on page number ` + strconv.Itoa(n) + `</a>`
		if n == lastPage {
			hrefs = `<a href="/posts/page-1-post">A post found on page number one</a>`
			next = 0
		}
		nextHTML := ""
		if next != 0 {
			nextHTML = `<a rel="next" href="/blog?page=` +
				strconv.Itoa(next) + `">Next</a>`
		}
		webFetch.SetHTML(page(n), `
			<html><body><main>`+hrefs+`</main>`+nextHTML+`</body></html>
		`)
	}

	s := NewFeedService(slog.Default(), nil, nil, webFetch, "", nil, nil, "")
	links, err := s.fetchPaginatedPostLinks(
		context.Background(), "https://example.com/blog", firstPage,
	)
	require.NoError(t, err)
	assert.Len(t, links, lastPage-1)
	wanted := []string{"/posts/page-1-post"}
	for n := 2; n < lastPage; n++ {
		wanted = append(wanted, "/posts/page-"+strconv.Itoa(n)+"-post")
	}
	urls := make([]string, 0, len(links))
	for _, l := range links {
		urls = append(urls, strings.TrimPrefix(l.URL, "https://example.com"))
	}
	assert.Equal(t, wanted, urls)
	assert.Len(t, webFetch.Calls, lastPage-1,
		"every pagination page up to the exhausted one must be fetched")
}

// A cycle of "next page" links ends via the visited set.
func TestFetchPaginatedPostLinksStopsOnPaginationLoop(t *testing.T) {
	webFetch := mocks.NewMockWebFetchClient()
	firstPage := []byte(`
		<html><body><main>
			<a href="/posts/page-1-post">A post found on page number one</a>
		</main>
		<a rel="next" href="/blog?page=2">Next</a>
		</body></html>
	`)
	webFetch.SetHTML("https://example.com/blog?page=2", `
		<html><body><main>
			<a href="/posts/page-2-post">A post discovered on page number two</a>
		</main>
		<a rel="next" href="/blog">Next</a>
		</body></html>
	`)

	s := NewFeedService(slog.Default(), nil, nil, webFetch, "", nil, nil, "")
	links, err := s.fetchPaginatedPostLinks(
		context.Background(), "https://example.com/blog", firstPage,
	)
	require.NoError(t, err)
	assert.Len(t, links, 2)
	assert.Equal(t, []string{"https://example.com/blog?page=2"}, webFetch.Calls)
}

func TestPageTitle(t *testing.T) {
	title := pageTitle([]byte(blogIndexHTML))
	assert.Equal(t, "Example Blog", title)
}

func TestPageTitleMissing(t *testing.T) {
	title := pageTitle([]byte("<html><body>no title here</body></html>"))
	assert.Equal(t, "", title)
}
