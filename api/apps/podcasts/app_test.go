package podcasts_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/feeds/pkg/webfetch"
	"tools.xdoubleu.com/apps/podcasts"
	"tools.xdoubleu.com/apps/podcasts/pkg/itunes"
	"tools.xdoubleu.com/gen/podcasts/v1/podcastsv1connect"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/logging"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

const (
	userID      = "podcasts-test-user"
	otherUserID = "podcasts-test-other-user"
)

//nolint:gochecknoglobals //needed for tests
var (
	testDB  postgres.DB
	testCfg config.Config
)

//nolint:gochecknoglobals //fixed fake catalog
var catalog = []itunes.Show{
	{
		ID: 1, Title: "Hardcore History", Author: "Dan Carlin",
		ArtworkURL: "https://img.example/1.jpg", FeedURL: "https://feeds.example/1",
		AppleURL: "https://podcasts.apple.com/podcast/id1",
	},
	{
		ID: 2, Title: "History of Rome", Author: "Mike Duncan",
		ArtworkURL: "", FeedURL: "https://feeds.example/2", AppleURL: "",
	},
	{
		ID: 3, Title: "Something else", Author: "",
		ArtworkURL: "", FeedURL: "https://feeds.example/3", AppleURL: "",
	},
}

// fakeITunes serves the fixed catalog.
type fakeITunes struct{}

func (fakeITunes) Search(_ context.Context, query string) ([]itunes.Show, error) {
	var out []itunes.Show
	for _, s := range catalog {
		if strings.Contains(strings.ToLower(s.Title), strings.ToLower(query)) {
			out = append(out, s)
		}
	}
	return out, nil
}

func (fakeITunes) Lookup(_ context.Context, id int64) (*itunes.Show, error) {
	for _, s := range catalog {
		if s.ID == id {
			return &s, nil
		}
	}
	return nil, itunes.ErrNotFound
}

// downITunes fails every call.
type downITunes struct{}

func (downITunes) Search(context.Context, string) ([]itunes.Show, error) {
	return nil, errors.New("itunes down")
}

func (downITunes) Lookup(context.Context, int64) (*itunes.Show, error) {
	return nil, errors.New("itunes down")
}

// fakeFeeds serves feed bodies by URL, honouring conditional GETs; unknown
// URLs answer 404.
type fakeFeeds struct {
	mu    sync.Mutex
	feeds map[string]*webfetch.Result
	errs  map[string]error
	calls []webfetch.Options
}

func newFakeFeeds() *fakeFeeds {
	//nolint:exhaustruct // the mutex starts zeroed
	return &fakeFeeds{
		feeds: map[string]*webfetch.Result{},
		errs:  map[string]error{},
		calls: nil,
	}
}

func (f *fakeFeeds) set(url string, body []byte, etag string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.feeds[url] = &webfetch.Result{
		Body: body, ContentType: "application/rss+xml", FinalURL: url,
		ETag: etag, LastModified: "", NotModified: false,
	}
}

func (f *fakeFeeds) Get(
	_ context.Context,
	rawURL string,
	opts webfetch.Options,
) (*webfetch.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, opts)
	if err := f.errs[rawURL]; err != nil {
		return nil, err
	}
	res, ok := f.feeds[rawURL]
	if !ok {
		return nil, &webfetch.StatusError{Code: http.StatusNotFound}
	}
	if opts.ETag != "" && opts.ETag == res.ETag {
		return &webfetch.Result{
			Body: nil, ContentType: "", FinalURL: rawURL,
			ETag: res.ETag, LastModified: "", NotModified: true,
		}, nil
	}
	return res, nil
}

func TestMain(m *testing.M) {
	testCfg = testhelper.NewTestConfig()
	pool := testhelper.ConnectTestDB(testCfg.DBDsn)
	testDB = pool

	ctx := context.Background()
	if _, err := testDB.Exec(ctx, "DROP SCHEMA IF EXISTS podcasts CASCADE"); err != nil {
		panic(err)
	}
	if err := newApp(userID, fakeITunes{}, nil).ApplyMigrations(ctx, pool); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

func newApp(
	uid string,
	client itunes.Client,
	fetch webfetch.Client,
) *podcasts.Podcasts {
	return podcasts.NewInner(
		sharedmocks.NewMockedAuthService(uid),
		logging.NewNopLogger(),
		testCfg,
		testDB,
		client,
		fetch,
	)
}

// newClient serves an app acting as uid; the user's favourites are emptied
// first.
func newClient(
	t *testing.T,
	uid string,
	client itunes.Client,
) podcastsv1connect.PodcastsServiceClient {
	t.Helper()
	return newClientWithFeeds(t, uid, client, newFakeFeeds())
}

// newClientWithFeeds is newClient with the given feed fetcher.
func newClientWithFeeds(
	t *testing.T,
	uid string,
	client itunes.Client,
	fetch webfetch.Client,
) podcastsv1connect.PodcastsServiceClient {
	t.Helper()
	_, err := testDB.Exec(context.Background(),
		"DELETE FROM podcasts.shows WHERE user_id = $1", uid)
	if err != nil {
		t.Fatal(err)
	}
	return newKeepingClient(t, uid, client, fetch)
}

// newKeepingClient serves an app acting as uid without clearing favourites.
func newKeepingClient(
	t *testing.T,
	uid string,
	client itunes.Client,
	fetch webfetch.Client,
) podcastsv1connect.PodcastsServiceClient {
	t.Helper()
	ts := httptest.NewServer(testhelper.BuildMux(newApp(uid, client, fetch)))
	t.Cleanup(ts.Close)
	return podcastsv1connect.NewPodcastsServiceClient(http.DefaultClient, ts.URL)
}

func TestAppMetadata(t *testing.T) {
	a := newApp(userID, nil, nil)
	assert.Equal(t, "podcasts", a.GetName())
	assert.Equal(t, "Podcasts", a.GetDisplayName())
	assert.NoError(t, a.Start())
}

func TestNew(t *testing.T) {
	a := podcasts.New(
		sharedmocks.NewMockedAuthService(userID), logging.NewNopLogger(), testCfg, testDB,
	)
	assert.NotNil(t, a)
}

func TestAllowPrivateFeeds_OnlyOutsideProduction(t *testing.T) {
	assert.False(t, podcasts.AllowPrivateFeeds(config.ProdEnv))
	assert.True(t, podcasts.AllowPrivateFeeds(config.DevEnv))
	assert.True(t, podcasts.AllowPrivateFeeds(""))
}
