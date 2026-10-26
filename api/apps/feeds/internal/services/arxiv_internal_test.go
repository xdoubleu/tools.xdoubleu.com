package services

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mmcdole/gofeed"
	ext "github.com/mmcdole/gofeed/extensions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/feeds/internal/mocks"
	"tools.xdoubleu.com/apps/feeds/internal/models"
)

func TestArxivID(t *testing.T) {
	cases := []struct {
		name, guid, link, want string
	}{
		{"api id with version", "http://arxiv.org/abs/2410.01234v2", "", "2410.01234"},
		{"oai guid", "oai:arXiv.org:2410.12345v1", "", "2410.12345"},
		{"old-style id", "", "https://arxiv.org/abs/math.GT/0309136v1", "math.GT/0309136"},
		{
			"old-style id without subclass",
			"",
			"http://arxiv.org/abs/hep-th/9901001",
			"hep-th/9901001",
		},
		{"falls back to link", "tag:x", "https://www.arxiv.org/abs/2410.01234", "2410.01234"},
		{"not arxiv", "https://example.com/abs/2410.01234", "", ""},
		{
			"arxiv path on another host",
			"https://evil.example/arxiv.org/abs/2410.01234",
			"",
			"",
		},
		{"arxiv lookalike host", "https://notarxiv.org/abs/2410.01234", "", ""},
		{"trailing junk", "https://arxiv.org/abs/2410.01234v1/../x", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			//nolint:exhaustruct // only GUID and Link are read
			item := &gofeed.Item{GUID: c.guid, Link: c.link}
			assert.Equal(t, c.want, arxivID(item))
		})
	}
}

func TestIsArxivFeed(t *testing.T) {
	assert.True(t, isArxivFeed("https://export.arxiv.org/api/query?search_query=x"))
	assert.True(t, isArxivFeed("https://RSS.arxiv.org/rss/cs.AI"))
	assert.False(t, isArxivFeed("https://arxiv.org/abs/2410.01234"))
	assert.False(t, isArxivFeed("https://example.com/feed"))
	assert.False(t, isArxivFeed(""))
	assert.False(t, isArxivFeed("://bad"))
}

func TestFeedItemGUID(t *testing.T) {
	//nolint:exhaustruct // only GUID and Link are read
	paper := &gofeed.Item{GUID: "oai:arXiv.org:2410.01234v3"}
	//nolint:exhaustruct // only Link is read
	post := &gofeed.Item{Link: "https://example.com/post"}
	//nolint:exhaustruct // only GUID and Extensions are read
	replaced := &gofeed.Item{
		GUID: "oai:arXiv.org:2410.09999v2",
		Extensions: ext.Extensions{"arxiv": {
			"announce_type": {{Value: " replace "}},
		}},
	}

	assert.Equal(t, "https://arxiv.org/abs/2410.01234", feedItemGUID(true, paper))
	assert.Equal(t, "oai:arXiv.org:2410.01234v3", feedItemGUID(false, paper))
	assert.Equal(t, "https://example.com/post", feedItemGUID(false, post))
	assert.Empty(t, feedItemGUID(true, post), "non-paper entries are skipped")
	assert.Empty(t, feedItemGUID(true, replaced))
}

func TestArxivAPIError(t *testing.T) {
	items := []*gofeed.Item{
		{GUID: "http://arxiv.org/abs/2410.01234v1"},
		{
			GUID:        "http://arxiv.org/api/errors#incorrect_id_format",
			Description: " incorrect id\n format ",
		},
	}
	assert.Equal(t, "incorrect id format", arxivAPIError(items))
	assert.Empty(t, arxivAPIError(items[:1]))
}

func TestSleepCtx(t *testing.T) {
	assert.NoError(t, sleepCtx(context.Background(), -time.Second))
	assert.NoError(t, sleepCtx(context.Background(), time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.ErrorIs(t, sleepCtx(ctx, time.Hour), context.Canceled)
}

func TestArxivItemHTML_OmitsMissingAuthorsAndAbstract(t *testing.T) {
	//nolint:exhaustruct // no authors or description
	item := &gofeed.Item{Authors: []*gofeed.Person{nil, {Name: "  "}}}
	assert.Equal(t,
		`<p><a href="https://arxiv.org/html/2410.01234">Read the paper (HTML)</a>`+
			` · <a href="https://arxiv.org/pdf/2410.01234">PDF</a></p>`,
		arxivItemHTML("2410.01234", item),
	)
}

func TestArxivAbstract(t *testing.T) {
	assert.Equal(t, "Plain text.", arxivAbstract("  Plain\n  text.  "))
	assert.Equal(t, "We note. Abstract: kept.", arxivAbstract("We note. Abstract: kept."))
	assert.Equal(t, "Body.",
		arxivAbstract("arXiv:2410.01234v1 Announce Type: new\nAbstract: Body."))
}

func TestIsFeedQuiet_ArxivFloorCoversWeekends(t *testing.T) {
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	// A busy feed whose last announcement was 72h ago, over a weekend.
	times := []time.Time{
		now.Add(-74 * time.Hour), now.Add(-73 * time.Hour), now.Add(-72 * time.Hour),
	}
	assert.True(t, isFeedQuiet(times, now, quietMinGap))
	assert.False(t, isFeedQuiet(times, now, arxivQuietMinGap))
}

func TestArxivPacer(t *testing.T) {
	var p arxivPacer
	assert.NoError(t, p.wait(context.Background(), "https://example.com/feed"))
	assert.True(t, p.last.IsZero(), "non-arXiv feeds don't count")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.NoError(t, p.wait(ctx, "https://rss.arxiv.org/rss/cs.AI"))
	assert.False(t, p.last.IsZero())

	short, cancelShort := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelShort()
	assert.ErrorIs(t, p.wait(short, "https://rss.arxiv.org/rss/cs.LG"),
		context.DeadlineExceeded, "a second arXiv poll waits for the gap")
}

func TestBuildItem_ArxivFeedUsesEntryWithoutFetching(t *testing.T) {
	webFetch := mocks.NewMockWebFetchClient()
	s := NewFeedService(
		slog.Default(), nil, nil, nil, nil, webFetch, "", nil, nil, "",
	)
	//nolint:exhaustruct // only the fields buildItem reads
	item := &gofeed.Item{
		Title:       "Paper",
		Link:        "https://arxiv.org/abs/2410.01234v2",
		Description: "Abstract text.",
	}
	//nolint:exhaustruct // buildItem only reads ID and URL off the feed
	feed := models.Feed{ID: uuid.New(), URL: "https://rss.arxiv.org/rss/cs.AI"}

	built, err := s.buildItem(context.Background(), feed, item, "g", nil)
	require.NoError(t, err)
	assert.Equal(t, "https://arxiv.org/abs/2410.01234", built.SourceURL)
	assert.Contains(t, built.ContentHTML, "<p>Abstract text.</p>")
	assert.Empty(t, webFetch.Calls)

	feed.URL = "https://example.com/feed"
	built, err = s.buildItem(context.Background(), feed, item, "g", nil)
	require.NoError(t, err)
	assert.Equal(t, "https://arxiv.org/abs/2410.01234v2", built.SourceURL)
}

func TestCreate_ArxivAPIErrorRejected(t *testing.T) {
	const feedURL = "https://export.arxiv.org/api/query?search_query=x"
	webFetch := mocks.NewMockWebFetchClient()
	webFetch.SetBody(feedURL, "application/atom+xml", []byte(
		`<feed xmlns="http://www.w3.org/2005/Atom"><entry>`+
			`<id>http://arxiv.org/api/errors#bad</id><summary>bad query</summary>`+
			`</entry></feed>`,
	))
	s := NewFeedService(
		slog.Default(), nil, nil, nil, nil, webFetch, "", nil, nil, "",
	)

	_, err := s.Create(context.Background(), "user", feedURL, "")
	require.ErrorIs(t, err, ErrInvalidFeed)
	assert.Contains(t, err.Error(), "bad query")
}
