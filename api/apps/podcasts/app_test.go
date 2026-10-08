package podcasts_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

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

func TestMain(m *testing.M) {
	testCfg = testhelper.NewTestConfig()
	pool := testhelper.ConnectTestDB(testCfg.DBDsn)
	testDB = pool

	ctx := context.Background()
	if _, err := testDB.Exec(ctx, "DROP SCHEMA IF EXISTS podcasts CASCADE"); err != nil {
		panic(err)
	}
	if err := newApp(userID, fakeITunes{}).ApplyMigrations(ctx, pool); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

func newApp(uid string, client itunes.Client) *podcasts.Podcasts {
	return podcasts.NewWithITunes(
		sharedmocks.NewMockedAuthService(uid),
		logging.NewNopLogger(),
		testCfg,
		testDB,
		client,
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
	_, err := testDB.Exec(context.Background(),
		"DELETE FROM podcasts.shows WHERE user_id = $1", uid)
	if err != nil {
		t.Fatal(err)
	}
	return newKeepingClient(t, uid, client)
}

// newKeepingClient serves an app acting as uid without clearing favourites.
func newKeepingClient(
	t *testing.T,
	uid string,
	client itunes.Client,
) podcastsv1connect.PodcastsServiceClient {
	t.Helper()
	ts := httptest.NewServer(testhelper.BuildMux(newApp(uid, client)))
	t.Cleanup(ts.Close)
	return podcastsv1connect.NewPodcastsServiceClient(http.DefaultClient, ts.URL)
}

func TestAppMetadata(t *testing.T) {
	a := newApp(userID, nil)
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
