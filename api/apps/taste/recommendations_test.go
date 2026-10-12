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

// wantedRefs records the refs pickCatalogs' WantBook was given.
//
//nolint:gochecknoglobals // test spy
var wantedRefs []string

func pickCatalogs() taste.Catalogs {
	found := func(title, creator string) *consumption.CatalogMatch {
		return &consumption.CatalogMatch{
			Ref: "ref-" + title, Title: title, Creator: creator, Year: 0,
			ImageURL: "https://img.example/" + title, URL: "https://cat.example/" + title,
			Known:        false,
			OnMyServices: false,
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
		WantBook: func(_ context.Context, _, ref string) error {
			wantedRefs = append(wantedRefs, ref)
			return nil
		},
		WantTitle: nil,
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
		&tastev1.ListRecommendationsRequest{Domain: domain, Track: track, SavedOnly: false},
	))
	require.NoError(t, err)
	return resp.Msg
}

func savedTitles(t *testing.T, c tastev1connect.TasteServiceClient) []string {
	t.Helper()
	resp, err := c.ListRecommendations(context.Background(), connect.NewRequest(
		&tastev1.ListRecommendationsRequest{Domain: "", Track: "", SavedOnly: true},
	))
	require.NoError(t, err)
	var out []string
	for _, r := range resp.Msg.Recommendations {
		out = append(out, r.Title)
	}
	return out
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
	id := insertPick(t, "Hidden")
	assert.Empty(t, listRecs(t, c, "", "").Recommendations)

	ctx := context.Background()
	_, err := c.SetSaved(ctx, connect.NewRequest(
		&tastev1.SetSavedRequest{Id: id, Saved: true},
	))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	_, err = testDB.Exec(ctx,
		"UPDATE taste.recommendations SET feedback = 'hit' WHERE id = $1", id)
	require.NoError(t, err)
	stats, err := c.GetStats(ctx, connect.NewRequest(&tastev1.GetStatsRequest{}))
	require.NoError(t, err)
	assert.Empty(t, stats.Msg.HitRates, "books hit rates need books access")
}

// insertPick stores a books pick as the latest run and returns its id.
func insertPick(t *testing.T, title string) string {
	t.Helper()
	ctx := context.Background()
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	_, err := testDB.Exec(ctx, `
		INSERT INTO taste.profiles (user_id, recommended_at) VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET recommended_at = $2`, userID, at)
	require.NoError(t, err)
	var id string
	require.NoError(t, testDB.QueryRow(ctx, `
		INSERT INTO taste.recommendations
		    (user_id, domain, track, title, score, generated_at)
		VALUES ($1, 'books', 'more', $2, 50, $3) RETURNING id`, userID, title, at,
	).Scan(&id))
	return id
}

func TestRecommendations_FeedbackSaveAccept(t *testing.T) {
	a, c := recsApp(t, sharedmocks.NewMockedAdminAuthService(userID))
	ctx := context.Background()
	require.NoError(t, a.GenerateProfilesNow(ctx))
	first := listRecs(t, c, "", "").Recommendations
	require.Len(t, first, 2)
	hyperion, radiolab := first[0], first[1]
	assert.Empty(t, hyperion.Feedback)
	assert.True(t, hyperion.CanAccept, "a book with a ref")
	assert.False(t, radiolab.CanAccept, "podcasts have no backlog")

	fb, err := c.SetFeedback(ctx, connect.NewRequest(
		&tastev1.SetFeedbackRequest{Id: hyperion.Id, Feedback: "hit"},
	))
	require.NoError(t, err)
	assert.Equal(t, "hit", fb.Msg.Recommendation.Feedback)
	saved, err := c.SetSaved(ctx, connect.NewRequest(
		&tastev1.SetSavedRequest{Id: hyperion.Id, Saved: true},
	))
	require.NoError(t, err)
	assert.True(t, saved.Msg.Recommendation.Saved)
	assert.Equal(t, []string{"Hyperion"}, savedTitles(t, c))

	stats, err := c.GetStats(ctx, connect.NewRequest(&tastev1.GetStatsRequest{}))
	require.NoError(t, err)
	require.Len(t, stats.Msg.HitRates, 1)
	assert.Equal(t, "books", stats.Msg.HitRates[0].Domain)
	assert.Equal(t, "more", stats.Msg.HitRates[0].Track)
	assert.Equal(t, int32(1), stats.Msg.HitRates[0].Hits)
	assert.Zero(t, stats.Msg.HitRates[0].Misses)

	// The next run drops Hyperion as a candidate, keeps the touched pick and
	// replaces the untouched Radiolab.
	_, err = testDB.Exec(ctx,
		"UPDATE taste.profiles SET pending = true WHERE user_id = $1", userID)
	require.NoError(t, err)
	require.NoError(t, a.GenerateProfilesNow(ctx))
	second := listRecs(t, c, "", "").Recommendations
	require.Len(t, second, 1)
	assert.Equal(t, "Radiolab", second[0].Title)
	assert.NotEqual(t, radiolab.Id, second[0].Id)
	assert.Equal(t, []string{"Hyperion"}, savedTitles(t, c))

	wantedRefs = nil
	acc, err := c.AcceptRecommendation(ctx, connect.NewRequest(
		&tastev1.AcceptRecommendationRequest{Id: hyperion.Id},
	))
	require.NoError(t, err)
	assert.True(t, acc.Msg.Recommendation.Accepted)
	assert.Equal(t, []string{"ref-Hyperion"}, wantedRefs)

	acc, err = c.AcceptRecommendation(ctx, connect.NewRequest(
		&tastev1.AcceptRecommendationRequest{Id: second[0].Id},
	))
	require.NoError(t, err)
	assert.True(t, acc.Msg.Recommendation.Saved, "no podcasts backlog: saved")
	assert.False(t, acc.Msg.Recommendation.Accepted)
	assert.Equal(t, []string{"Hyperion", "Radiolab"}, savedTitles(t, c))
}

func TestRecommendations_MutationErrors(t *testing.T) {
	_, c := recsApp(t, sharedmocks.NewMockedAdminAuthService(userID))
	ctx := context.Background()
	id := insertPick(t, "Known")

	_, err := c.SetFeedback(ctx, connect.NewRequest(
		&tastev1.SetFeedbackRequest{Id: id, Feedback: "meh"},
	))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = c.SetSaved(ctx, connect.NewRequest(
		&tastev1.SetSavedRequest{Id: "nope", Saved: true},
	))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = c.AcceptRecommendation(ctx, connect.NewRequest(
		&tastev1.AcceptRecommendationRequest{Id: "00000000-0000-0000-0000-000000000000"},
	))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	cleared, err := c.SetFeedback(ctx, connect.NewRequest(
		&tastev1.SetFeedbackRequest{Id: id, Feedback: ""},
	))
	require.NoError(t, err)
	assert.Empty(t, cleared.Msg.Recommendation.Feedback)
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

func TestGetStats_HitRateReadFails(t *testing.T) {
	pool := testhelper.ConnectTestDB(testCfg.DBDsn)
	a := taste.NewInner(sharedmocks.NewMockedAdminAuthService(userID),
		logging.NewNopLogger(), testCfg, pool, pickLLM{}, pickCatalogs(),
		userSource{name: "books", events: []consumption.Event{book}, err: nil},
	)
	ts := httptest.NewServer(testhelper.BuildMux(a))
	t.Cleanup(ts.Close)
	c := tastev1connect.NewTasteServiceClient(http.DefaultClient, ts.URL)
	pool.Close()

	_, err := c.GetStats(context.Background(),
		connect.NewRequest(&tastev1.GetStatsRequest{}))
	assert.Error(t, err)
}
