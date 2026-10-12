package taste_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/taste"
	tastev1 "tools.xdoubleu.com/gen/taste/v1"
	"tools.xdoubleu.com/gen/taste/v1/tastev1connect"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/consumption"
	"tools.xdoubleu.com/internal/logging"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/openrouter"
	"tools.xdoubleu.com/internal/testhelper"
)

// pickLLM writes the profile, proposes two picks and scores them.
type pickLLM struct{}

func (pickLLM) Complete(
	_ context.Context, req openrouter.Request,
) (*openrouter.Response, error) {
	content := profileJSON
	switch req.SchemaName {
	case "taste_candidates":
		content = `{"picks": [
			{"domain": "books", "track": "more", "title": "Hyperion", "creator": "Dan Simmons",
			 "year": 1989, "url": "", "why": "Like Dune.", "neighbour": "purist"},
			{"domain": "podcasts", "track": "broaden", "title": "Radiolab", "creator": "",
			 "year": 0, "url": "", "why": "Big ideas, by ear.", "neighbour": "explorer"}
		]}`
	case "taste_scores":
		content = `{"scores": [{"id": 0, "score": 88}, {"id": 1, "score": 61}]}`
	}
	return &openrouter.Response{
		Model: "served/model", Content: content,
		Citations: []openrouter.Citation{
			{URL: "https://src.example/hyperion", Title: "Hyperion, reviewed"},
		},
		Usage: openrouter.Usage{PromptTokens: 0, CompletionTokens: 0, Cost: 0},
	}, nil
}

func pickCatalogs() taste.Catalogs {
	found := func(title, creator string) *consumption.CatalogMatch {
		return &consumption.CatalogMatch{
			Ref: "ref-" + title, Title: title, Creator: creator, Year: 0,
			ImageURL: "https://img.example/" + title, URL: "https://cat.example/" + title,
			Known: false,
		}
	}
	return taste.Catalogs{
		Books: func(
			_ context.Context, _, title, author string,
		) (*consumption.CatalogMatch, error) {
			return found(title, author), nil
		},
		Movies: nil,
		Games:  nil,
		Podcasts: func(
			_ context.Context, _, title string,
		) (*consumption.CatalogMatch, error) {
			return found(title, "WNYC"), nil
		},
		Feeds: nil,
	}
}

func recsApp(
	t *testing.T,
	authService auth.Service,
) (*taste.Taste, tastev1connect.TasteServiceClient) {
	t.Helper()
	ctx := context.Background()
	for _, table := range []string{"taste.recommendations", "taste.profiles"} {
		_, err := testDB.Exec(ctx, "DELETE FROM "+table+" WHERE user_id = $1", userID)
		require.NoError(t, err)
	}
	cfg := testCfg
	cfg.OpenRouterAPIKey = "test-key"
	a := taste.NewInner(authService, logging.NewNopLogger(), cfg, testDB, pickLLM{},
		pickCatalogs(),
		userSource{name: "books", events: []consumption.Event{book}, err: nil},
	)
	ts := httptest.NewServer(testhelper.BuildMux(a))
	t.Cleanup(ts.Close)
	return a, tastev1connect.NewTasteServiceClient(http.DefaultClient, ts.URL)
}

func listRecs(
	t *testing.T,
	c tastev1connect.TasteServiceClient,
	domain, track string,
) *tastev1.ListRecommendationsResponse {
	t.Helper()
	resp, err := c.ListRecommendations(context.Background(), connect.NewRequest(
		&tastev1.ListRecommendationsRequest{Domain: domain, Track: track},
	))
	require.NoError(t, err)
	return resp.Msg
}

func TestRecommendations(t *testing.T) {
	a, c := recsApp(t, sharedmocks.NewMockedAdminAuthService(userID))

	before := listRecs(t, c, "", "")
	assert.Empty(t, before.Recommendations)
	assert.Empty(t, before.GeneratedAt)

	require.NoError(t, a.GenerateProfilesNow(context.Background()))

	all := listRecs(t, c, "", "")
	require.Len(t, all.Recommendations, 2)
	assert.Empty(t, all.LastError)
	at, err := time.Parse(time.RFC3339, all.GeneratedAt)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now(), at, time.Minute)

	r := all.Recommendations[0]
	assert.NotEmpty(t, r.Id)
	assert.Equal(t, "books", r.Domain)
	assert.Equal(t, "more", r.Track)
	assert.Equal(t, "Hyperion", r.Title)
	assert.Equal(t, "Dan Simmons", r.Creator)
	assert.Equal(t, int32(1989), r.Year)
	assert.Equal(t, "https://cat.example/Hyperion", r.Url)
	assert.Equal(t, "https://img.example/Hyperion", r.ImageUrl)
	assert.Equal(t, "Like Dune.", r.Why)
	assert.Equal(t, int32(88), r.Score)
	require.Len(t, r.Sources, 1)
	assert.Equal(t, "https://src.example/hyperion", r.Sources[0].Url)
	assert.Equal(t, "Hyperion, reviewed", r.Sources[0].Title)

	podcasts := listRecs(t, c, "podcasts", "broaden")
	require.Len(t, podcasts.Recommendations, 1)
	assert.Equal(t, "Radiolab", podcasts.Recommendations[0].Title)
	assert.Empty(t, listRecs(t, c, "podcasts", "more").Recommendations)
}

func TestRecommendations_HiddenWithoutAppAccess(t *testing.T) {
	// The mocked non-admin user may open neither books nor podcasts.
	_, c := recsApp(t, sharedmocks.NewMockedAuthService(userID))
	_, err := testDB.Exec(context.Background(), `
		INSERT INTO taste.recommendations
		    (user_id, domain, track, title, score, generated_at)
		VALUES ($1, 'books', 'more', 'Hidden', 50, now())`, userID)
	require.NoError(t, err)
	assert.Empty(t, listRecs(t, c, "", "").Recommendations)
}

func TestRecommendations_InvalidArguments(t *testing.T) {
	c := newClient(t)
	for _, req := range []*tastev1.ListRecommendationsRequest{
		{Domain: "music", Track: ""},
		{Domain: "", Track: "sideways"},
	} {
		_, err := c.ListRecommendations(context.Background(), connect.NewRequest(req))
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	}
}
