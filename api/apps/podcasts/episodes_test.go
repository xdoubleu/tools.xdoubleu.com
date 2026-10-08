package podcasts_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/feeds/pkg/webfetch"
	"tools.xdoubleu.com/apps/podcasts/pkg/itunes"
	podcastsv1 "tools.xdoubleu.com/gen/podcasts/v1"
	"tools.xdoubleu.com/gen/podcasts/v1/podcastsv1connect"
)

const (
	feedURL = "https://feeds.example/1"
	romeURL = "https://feeds.example/2"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/feed.xml")
	require.NoError(t, err)
	return body
}

func episodes(
	t *testing.T,
	c podcastsv1connect.PodcastsServiceClient,
	req *podcastsv1.ListEpisodesRequest,
) *podcastsv1.ListEpisodesResponse {
	t.Helper()
	resp, err := c.ListEpisodes(context.Background(), connect.NewRequest(req))
	require.NoError(t, err)
	return resp.Msg
}

func titles(eps []*podcastsv1.Episode) []string {
	out := make([]string, len(eps))
	for i, e := range eps {
		out[i] = e.Title
	}
	return out
}

func TestAddFavourite_FetchesEpisodes(t *testing.T) {
	feeds := newFakeFeeds()
	feeds.set(feedURL, fixture(t), `"v1"`)
	c := newClientWithFeeds(t, userID, fakeITunes{}, feeds)

	f := add(t, c, 1)
	assert.Empty(t, f.FetchError)

	got := episodes(t, c, &podcastsv1.ListEpisodesRequest{}).Episodes
	require.Len(t, got, 3)
	// Newest first; the undated bonus episode sorts last.
	assert.Equal(t, []string{
		"Show 70 - Supernova in the East I",
		"Show 69 - The Celtic Holocaust",
		"Bonus - no guid, no date",
	}, titles(got))

	first := got[0]
	assert.Equal(t, f.Id, first.ShowId)
	assert.Equal(t, "Hardcore History", first.ShowTitle)
	assert.Equal(t, "https://img.example/1.jpg", first.ArtworkUrl)
	assert.Equal(t, "https://podcasts.apple.com/podcast/id1", first.AppleUrl)
	assert.Equal(t, "https://shows.example/hh/70", first.Link)
	assert.Equal(t, "https://cdn.example/hh70.mp3", first.AudioUrl)
	assert.Equal(t, "The Pacific war & its roots. Part one.", first.Summary)
	require.NotNil(t, first.DurationSeconds)
	assert.Equal(t, int32(3723), *first.DurationSeconds)
	published, err := time.Parse(time.RFC3339, first.PublishedAt)
	require.NoError(t, err)
	assert.True(t, published.Equal(time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)))

	// "3725" seconds; the bonus has an unparseable duration and no date.
	require.NotNil(t, got[1].DurationSeconds)
	assert.Equal(t, int32(3725), *got[1].DurationSeconds)
	assert.Nil(t, got[2].DurationSeconds)
	assert.Empty(t, got[2].PublishedAt)
	assert.Equal(t, "https://cdn.example/bonus.mp3", got[2].AudioUrl)
}

func TestAddFavourite_FeedFailureIsRecordedNotFatal(t *testing.T) {
	tests := map[string]struct {
		setup func(*fakeFeeds)
		want  string
	}{
		"http status": {func(*fakeFeeds) {}, "feed answered HTTP 404"},
		"unreachable": {
			func(f *fakeFeeds) { f.errs[feedURL] = webfetch.ErrNetwork },
			"couldn't reach the feed",
		},
		"too large": {
			func(f *fakeFeeds) { f.errs[feedURL] = webfetch.ErrTooLarge },
			"feed is too large",
		},
		"not rss": {
			func(f *fakeFeeds) { f.set(feedURL, []byte("<html>nope</html>"), "") },
			"feed isn't valid RSS",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			feeds := newFakeFeeds()
			tt.setup(feeds)
			c := newClientWithFeeds(t, userID, fakeITunes{}, feeds)

			f := add(t, c, 1)
			assert.Empty(t, f.FetchError, "the response predates the poll")
			assert.Equal(t, tt.want, favourites(t, c)[0].FetchError)
			assert.Empty(t, episodes(t, c, &podcastsv1.ListEpisodesRequest{}).Episodes)
		})
	}
}

func TestRepoll_RefreshesWithoutDuplicatesAndUsesValidators(t *testing.T) {
	feeds := newFakeFeeds()
	feeds.set(feedURL, fixture(t), `"v1"`)
	c := newClientWithFeeds(t, userID, fakeITunes{}, feeds)
	add(t, c, 1)

	// Unchanged feed: the second fetch is conditional and answers 304.
	add(t, c, 1)
	require.Len(t, feeds.calls, 2)
	assert.Empty(t, feeds.calls[0].ETag)
	assert.Equal(t, `"v1"`, feeds.calls[1].ETag)
	assert.Len(t, episodes(t, c, &podcastsv1.ListEpisodesRequest{}).Episodes, 3)

	// A changed feed fixes a title and adds an episode without duplicating.
	changed := append([]byte(nil), fixture(t)...)
	changed = replaceOnce(changed, "Show 69 - The Celtic Holocaust", "Show 69 - Fixed")
	const newest = `<item><title>Show 71</title><guid>hh-71</guid>
		<pubDate>Mon, 09 Nov 2026 08:00:00 +0000</pubDate></item></channel>`
	changed = replaceOnce(changed, "</channel>", newest)
	feeds.set(feedURL, changed, `"v2"`)
	add(t, c, 1)

	got := titles(episodes(t, c, &podcastsv1.ListEpisodesRequest{}).Episodes)
	assert.Equal(t, []string{
		"Show 71", "Show 70 - Supernova in the East I", "Show 69 - Fixed",
		"Bonus - no guid, no date",
	}, got)
}

func TestFailedRepollKeepsEpisodesAndRecoversWhenFixed(t *testing.T) {
	feeds := newFakeFeeds()
	feeds.set(feedURL, fixture(t), `"v1"`)
	c := newClientWithFeeds(t, userID, fakeITunes{}, feeds)
	add(t, c, 1)

	feeds.errs[feedURL] = webfetch.ErrNetwork
	add(t, c, 1)
	assert.Equal(t, "couldn't reach the feed", favourites(t, c)[0].FetchError)
	assert.Len(t, episodes(t, c, &podcastsv1.ListEpisodesRequest{}).Episodes, 3)

	delete(feeds.errs, feedURL)
	add(t, c, 1)
	assert.Empty(t, favourites(t, c)[0].FetchError)
	// The failed poll kept the last good validators.
	assert.Equal(t, `"v1"`, feeds.calls[len(feeds.calls)-1].ETag)
}

func TestListEpisodes_FilterAndPaging(t *testing.T) {
	feeds := newFakeFeeds()
	feeds.set(feedURL, fixture(t), "")
	feeds.set(romeURL, []byte(`<rss version="2.0"><channel><title>Rome</title>
		<item><title>Rome 1</title><guid>r1</guid>
		<pubDate>Tue, 06 Oct 2026 08:00:00 +0000</pubDate></item></channel></rss>`), "")
	c := newClientWithFeeds(t, userID, fakeITunes{}, feeds)
	hh := add(t, c, 1)
	rome := add(t, c, 2)

	all := episodes(t, c, &podcastsv1.ListEpisodesRequest{})
	assert.Equal(t, "Rome 1", all.Episodes[0].Title, "newest across shows first")
	assert.Len(t, all.Episodes, 4)
	assert.False(t, all.HasMore)

	only := episodes(t, c, &podcastsv1.ListEpisodesRequest{ShowId: hh.Id})
	assert.Len(t, only.Episodes, 3)
	only = episodes(t, c, &podcastsv1.ListEpisodesRequest{ShowId: rome.Id})
	assert.Equal(t, []string{"Rome 1"}, titles(only.Episodes))

	page := episodes(t, c, &podcastsv1.ListEpisodesRequest{Limit: 2})
	assert.Equal(t, []string{"Rome 1", "Show 70 - Supernova in the East I"},
		titles(page.Episodes))
	assert.True(t, page.HasMore)
	page = episodes(t, c, &podcastsv1.ListEpisodesRequest{Limit: 2, Offset: 2})
	assert.Equal(t, []string{
		"Show 69 - The Celtic Holocaust", "Bonus - no guid, no date",
	}, titles(page.Episodes))
	assert.False(t, page.HasMore)
}

func TestListEpisodes_InvalidShowID(t *testing.T) {
	c := newClient(t, userID, fakeITunes{})

	_, err := c.ListEpisodes(context.Background(), connect.NewRequest(
		&podcastsv1.ListEpisodesRequest{ShowId: "nope"},
	))
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
}

func TestListEpisodes_ArePerUser(t *testing.T) {
	feeds := newFakeFeeds()
	feeds.set(feedURL, fixture(t), "")
	mine := newClientWithFeeds(t, userID, fakeITunes{}, feeds)
	other := newClientWithFeeds(t, otherUserID, fakeITunes{}, feeds)
	f := add(t, mine, 1)

	assert.Empty(t, episodes(t, other, &podcastsv1.ListEpisodesRequest{}).Episodes)
	assert.Empty(t, episodes(t, other,
		&podcastsv1.ListEpisodesRequest{ShowId: f.Id}).Episodes)
}

func TestRemoveFavourite_DropsItsEpisodes(t *testing.T) {
	feeds := newFakeFeeds()
	feeds.set(feedURL, fixture(t), "")
	c := newClientWithFeeds(t, userID, fakeITunes{}, feeds)
	f := add(t, c, 1)

	_, err := c.RemoveFavourite(context.Background(), connect.NewRequest(
		&podcastsv1.RemoveFavouriteRequest{Id: f.Id},
	))
	require.NoError(t, err)
	assert.Empty(t, episodes(t, c, &podcastsv1.ListEpisodesRequest{}).Episodes)
	var n int
	require.NoError(t, testDB.QueryRow(context.Background(),
		"SELECT count(*) FROM podcasts.episodes e WHERE NOT EXISTS ("+
			"SELECT 1 FROM podcasts.shows s WHERE s.id = e.show_id)").Scan(&n))
	assert.Zero(t, n)
}

func TestPollNow_PollsEveryUsersFavourites(t *testing.T) {
	feeds := newFakeFeeds()
	mine := newClientWithFeeds(t, userID, fakeITunes{}, feeds)
	other := newClientWithFeeds(t, otherUserID, fakeITunes{}, feeds)
	// Both add while the feed is missing, so they start without episodes.
	add(t, mine, 1)
	add(t, other, 1)
	assert.Empty(t, episodes(t, mine, &podcastsv1.ListEpisodesRequest{}).Episodes)

	feeds.set(feedURL, fixture(t), "")
	feeds.errs[romeURL] = errors.New("unused")
	require.NoError(t,
		newApp(userID, fakeITunes{}, feeds).PollNow(context.Background()))

	assert.Len(t, episodes(t, mine, &podcastsv1.ListEpisodesRequest{}).Episodes, 3)
	assert.Len(t, episodes(t, other, &podcastsv1.ListEpisodesRequest{}).Episodes, 3)
	assert.Empty(t, favourites(t, mine)[0].FetchError)
}

func TestPollNow_OneFailingShowDoesNotStopTheRest(t *testing.T) {
	feeds := newFakeFeeds()
	c := newClientWithFeeds(t, userID, fakeITunes{}, feeds)
	add(t, c, 1)
	add(t, c, 2)

	feeds.errs[feedURL] = webfetch.ErrNetwork
	feeds.set(romeURL, []byte(`<rss version="2.0"><channel>
		<item><title>Rome 1</title><guid>r1</guid></item></channel></rss>`), "")
	require.NoError(t,
		newApp(userID, fakeITunes{}, feeds).PollNow(context.Background()))

	assert.Equal(t, []string{"Rome 1"},
		titles(episodes(t, c, &podcastsv1.ListEpisodesRequest{}).Episodes))
}

func replaceOnce(body []byte, from, to string) []byte {
	return []byte(strings.Replace(string(body), from, to, 1))
}

// movedITunes lists the first show under a new feed URL.
type movedITunes struct{ fakeITunes }

const movedURL = "https://feeds.example/1-moved"

func (m movedITunes) Lookup(ctx context.Context, id int64) (*itunes.Show, error) {
	show, err := m.fakeITunes.Lookup(ctx, id)
	if show != nil && id == 1 {
		show.FeedURL = movedURL
	}
	return show, err
}

func TestAddFavourite_NewFeedURLDropsTheOldValidators(t *testing.T) {
	feeds := newFakeFeeds()
	feeds.set(feedURL, fixture(t), `"v1"`)
	feeds.set(movedURL, fixture(t), `"v1"`)
	c := newClientWithFeeds(t, userID, fakeITunes{}, feeds)
	add(t, c, 1)

	moved := newKeepingClient(t, userID, movedITunes{fakeITunes: fakeITunes{}}, feeds)
	add(t, moved, 1)

	require.Len(t, feeds.calls, 2)
	assert.Empty(t, feeds.calls[0].ETag)
	assert.Empty(t, feeds.calls[1].ETag, "the old URL's ETag isn't sent to the new URL")
	assert.Empty(t, favourites(t, c)[0].FetchError)
}

func TestAddFavourite_UnchangedFeedURLKeepsItsValidators(t *testing.T) {
	feeds := newFakeFeeds()
	feeds.set(feedURL, fixture(t), `"v1"`)
	c := newClientWithFeeds(t, userID, fakeITunes{}, feeds)
	add(t, c, 1)
	add(t, c, 1)

	require.Len(t, feeds.calls, 2)
	assert.Equal(t, `"v1"`, feeds.calls[1].ETag)
}

func TestAddFavourite_StorageFailureIsRecorded(t *testing.T) {
	_, err := testDB.Exec(context.Background(), `ALTER TABLE podcasts.episodes
		ADD CONSTRAINT reject_test CHECK (title <> 'Reject me')`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			"ALTER TABLE podcasts.episodes DROP CONSTRAINT reject_test")
	})
	feeds := newFakeFeeds()
	feeds.set(feedURL, []byte(`<rss version="2.0"><channel>
		<item><title>Reject me</title><guid>x</guid></item></channel></rss>`), `"v1"`)
	c := newClientWithFeeds(t, userID, fakeITunes{}, feeds)

	add(t, c, 1)
	assert.Equal(t, "couldn't store the episodes", favourites(t, c)[0].FetchError)
}
