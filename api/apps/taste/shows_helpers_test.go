package taste_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"

	"tools.xdoubleu.com/apps/taste"
	"tools.xdoubleu.com/apps/taste/pkg/itunes"
	"tools.xdoubleu.com/gen/taste/v1/tastev1connect"
	"tools.xdoubleu.com/internal/kit/logging"
	"tools.xdoubleu.com/internal/kit/testhelper"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
)

const (
	showsUserID      = "taste-shows-test-user"
	showsOtherUserID = "taste-shows-test-other-user"
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
		ArtworkURL: "", FeedURL: "https://feeds.example/x", AppleURL: "",
	},
	{
		ID: 3, Title: "Something else", Author: "",
		ArtworkURL: "", FeedURL: "https://feeds.example/x", AppleURL: "",
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

// newShowsApp is an admin's app over the given iTunes client.
func newShowsApp(uid string, client itunes.Client) *taste.Taste {
	return taste.NewInner(
		context.Background(),
		sharedmocks.NewMockedAdminAuthService(uid),
		logging.NewNopLogger(),
		testCfg,
		testDB,
		testFamily,
		newLLM(""),
		nil,
		noCatalogs(),
	).WithITunes(client)
}

// newShowsClient serves an app acting as uid; the user's favourite shows are
// emptied before and after.
func newShowsClient(
	t *testing.T,
	uid string,
	client itunes.Client,
) tastev1connect.TasteServiceClient {
	t.Helper()
	clean := func() {
		_, err := testDB.Exec(context.Background(),
			"DELETE FROM podcasts.shows WHERE user_id = $1", uid)
		if err != nil {
			t.Fatal(err)
		}
	}
	clean()
	t.Cleanup(clean)
	ts := httptest.NewServer(testhelper.BuildMux(newShowsApp(uid, client)))
	t.Cleanup(ts.Close)
	return tastev1connect.NewTasteServiceClient(
		connect.NewClient(
			connecthttp.NewTransport(
				http.DefaultClient,
				ts.URL,
				connecthttp.WithReadMaxBytes(0),
			),
		),
	)
}
