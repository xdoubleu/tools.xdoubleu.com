package services

import (
	"compress/gzip"
	"embed"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// uberLiveFixtures holds real, captured HTML of Uber's engineering blog index
// (https://www.uber.com/be/en/blog/engineering and its /page/2/), fetched the
// way the production scraper sees it. The scrape heuristic has repeatedly
// failed in ways synthetic fixtures never reproduced (issue #1748), so these
// tests pin its behavior against the actual page bytes — when the live
// site's shape is relevant to a change, recapture the fixtures rather than
// hand-editing them.
//
//go:embed testdata/uber-engineering-page*.html.gz
var uberLiveFixtures embed.FS

func readUberFixture(t *testing.T, name string) []byte {
	t.Helper()
	f, err := uberLiveFixtures.Open("testdata/" + name)
	require.NoError(t, err)
	defer f.Close()
	zr, err := gzip.NewReader(f)
	require.NoError(t, err)
	body, err := io.ReadAll(zr)
	require.NoError(t, err)
	return body
}

// localeSwitcherTitles are the language-switcher anchor texts that leaked
// into the feed as bogus posts (issue #1748).
//
//nolint:gochecknoglobals // static fixture table, read-only after init
var localeSwitcherTitles = []string{
	"English, English", "French, Français (France)",
	"German, Deutsch", "Dutch, Nederlands",
}

func assertNoLocaleSwitcherLinks(t *testing.T, links []discoveredLink) {
	t.Helper()
	for _, link := range links {
		for _, junk := range localeSwitcherTitles {
			assert.NotEqual(t, junk, link.Title,
				"locale switcher leaked as post link %s", link.URL)
		}
	}
}

func assertContainsURL(t *testing.T, links []discoveredLink, want string) {
	t.Helper()
	for _, link := range links {
		if link.URL == want {
			return
		}
	}
	t.Errorf("expected post link %s, got %v", want, links)
}

func TestDiscoverPostLinksUberEngineeringLivePage1(t *testing.T) {
	links, err := discoverPostLinks(
		"https://www.uber.com/be/en/blog/engineering/",
		readUberFixture(t, "uber-engineering-page1.html.gz"),
	)
	require.NoError(t, err)
	assertNoLocaleSwitcherLinks(t, links)
	assertContainsURL(t, links, "https://www.uber.com/be/en/blog/signals-to-context/")

	_, ok := discoverNextPageURL(
		"https://www.uber.com/be/en/blog/engineering/",
		readUberFixture(t, "uber-engineering-page1.html.gz"),
	)
	assert.True(t, ok, "page 1's Next link must be discovered for pagination")
}

func TestDiscoverPostLinksUberEngineeringLivePage2(t *testing.T) {
	links, err := discoverPostLinks(
		"https://www.uber.com/be/en/blog/engineering/page/2/",
		readUberFixture(t, "uber-engineering-page2.html.gz"),
	)
	require.NoError(t, err)
	assertNoLocaleSwitcherLinks(t, links)
	assertContainsURL(t, links, "https://www.uber.com/be/en/blog/junit-migration/")
}
