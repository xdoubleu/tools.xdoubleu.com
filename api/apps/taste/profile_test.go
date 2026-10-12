package taste_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/taste"
	tastev1 "tools.xdoubleu.com/gen/taste/v1"
	"tools.xdoubleu.com/gen/taste/v1/tastev1connect"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/consumption"
	"tools.xdoubleu.com/internal/logging"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

const profileJSON = `{
	"summary": "You love big ideas in science fiction.",
	"know": ["Space opera"],
	"like": ["Ambitious world-building", "b", "c", "d", "e", "f", "g"],
	"drifting": [],
	"bounce": ["Short survival games"]
}`

// profileApp is the app with an OpenRouter key and llm, as authService's
// user, over the book source; the user's profile row is cleared first.
func profileApp(
	t *testing.T,
	authService auth.Service,
	llm *fakeLLM,
) (*taste.Taste, tastev1connect.TasteServiceClient) {
	t.Helper()
	_, err := testDB.Exec(context.Background(),
		"DELETE FROM taste.profiles WHERE user_id = $1", userID)
	require.NoError(t, err)

	cfg := testCfg
	cfg.OpenRouterAPIKey = "test-key"
	a := taste.NewInner(
		authService,
		logging.NewNopLogger(),
		cfg,
		testDB,
		llm,
		noCatalogs(),
		userSource{name: "books", events: []consumption.Event{book}, err: nil},
	)
	ts := httptest.NewServer(testhelper.BuildMux(a))
	t.Cleanup(ts.Close)
	return a, tastev1connect.NewTasteServiceClient(
		connect.NewClient(
			connecthttp.NewTransport(
				http.DefaultClient,
				ts.URL,
				connecthttp.WithReadMaxBytes(0),
			),
		),
	)
}

func getProfile(
	t *testing.T,
	c tastev1connect.TasteServiceClient,
) *tastev1.GetProfileResponse {
	t.Helper()
	resp, err := c.GetProfile(context.Background(), &tastev1.GetProfileRequest{})
	require.NoError(t, err)
	return resp
}

func refresh(c tastev1connect.TasteServiceClient) error {
	_, err := c.RefreshProfile(context.Background(), &tastev1.RefreshProfileRequest{})
	return err
}

func TestProfile_NotConfigured(t *testing.T) {
	c := newClient(t)
	p := getProfile(t, c)
	assert.False(t, p.Configured)
	assert.Nil(t, p.Sections)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(refresh(c)))
}

func TestProfile_RefreshGeneratesAndRateLimits(t *testing.T) {
	llm := newLLM(profileJSON)
	a, c := profileApp(t, sharedmocks.NewMockedAdminAuthService(userID), llm)

	p := getProfile(t, c)
	assert.True(t, p.Configured)
	assert.Nil(t, p.Sections)
	assert.False(t, p.Pending)
	assert.Empty(t, p.NextRefreshAt)

	require.NoError(t, refresh(c))
	err := refresh(c)
	assert.Equal(t, connect.CodeResourceExhausted, connect.CodeOf(err))
	assert.ErrorContains(t, err, "refreshed in the last 6h0m0s")
	p = getProfile(t, c)
	assert.True(t, p.Pending)
	var next time.Time
	next, err = time.Parse(time.RFC3339, p.NextRefreshAt)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(6*time.Hour), next, time.Minute)

	require.NoError(t, a.GenerateProfilesNow(context.Background()))
	p = getProfile(t, c)
	assert.False(t, p.Pending)
	assert.Empty(t, p.LastError)
	assert.Equal(t, "served/model", p.Model)
	require.NotNil(t, p.Sections)
	assert.Equal(t, "You love big ideas in science fiction.", p.Sections.Summary)
	assert.Equal(t, []string{"Space opera"}, p.Sections.Know)
	assert.Len(t, p.Sections.Like, 6, "capped at six bullets")
	assert.Empty(t, p.Sections.Drifting)
	assert.Equal(t, []string{"Short survival games"}, p.Sections.Bounce)
	generated, err := time.Parse(time.RFC3339, p.GeneratedAt)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now(), generated, time.Minute)

	require.Equal(t, 1, llm.profileCalls())
	req := llm.requests[0]
	assert.Equal(t, testCfg.TasteGeneratorModel, req.Model)
	assert.Equal(t, "taste_profile", req.SchemaName)
	assert.Nil(t, req.WebSearch)
	require.Len(t, req.Messages, 2)
	assert.Contains(t, req.Messages[1].Content, "Dune — Frank Herbert [sci-fi]")

	// Fresh, so the next run leaves it alone.
	require.NoError(t, a.GenerateProfilesNow(context.Background()))
	assert.Equal(t, 1, llm.profileCalls())
}

func TestProfile_FailureKeepsLastProfile(t *testing.T) {
	llm := newLLM(profileJSON)
	a, c := profileApp(t, sharedmocks.NewMockedAdminAuthService(userID), llm)
	require.NoError(t, a.GenerateProfilesNow(context.Background()))

	_, err := testDB.Exec(context.Background(), `
		UPDATE taste.profiles SET pending = true, requested_at = NULL
		WHERE user_id = $1`, userID)
	require.NoError(t, err)
	llm.err = errors.New("openrouter: status 402: credit limit for key sk-123")
	require.ErrorContains(t, a.GenerateProfilesNow(context.Background()), "status 402")

	p := getProfile(t, c)
	assert.Equal(t, "The model service failed.", p.LastError, "details stay in logs")
	assert.False(t, p.Pending)
	require.NotNil(t, p.Sections, "the previous profile stays")

	// Failed attempts count too, so it isn't retried hourly.
	require.NoError(t, a.GenerateProfilesNow(context.Background()))
	assert.Equal(t, 2, llm.profileCalls())
}

func TestProfile_InvalidAnswers(t *testing.T) {
	for _, content := range []string{`not json`, `{"summary": "  "}`} {
		llm := newLLM(content)
		a, c := profileApp(t, sharedmocks.NewMockedAdminAuthService(userID), llm)
		require.Error(t, a.GenerateProfilesNow(context.Background()))
		p := getProfile(t, c)
		assert.Equal(t, "The model's answer couldn't be read.", p.LastError)
		assert.Nil(t, p.Sections)
	}
}

func TestProfile_SourceFailureIsNoted(t *testing.T) {
	_, err := testDB.Exec(context.Background(),
		"DELETE FROM taste.profiles WHERE user_id = $1", userID)
	require.NoError(t, err)
	cfg := testCfg
	cfg.OpenRouterAPIKey = "test-key"
	llm := newLLM(profileJSON)
	a := taste.NewInner(sharedmocks.NewMockedAdminAuthService(userID),
		logging.NewNopLogger(), cfg, testDB, llm, noCatalogs(),
		userSource{name: "books", events: nil, err: errors.New("db down")})
	require.ErrorContains(t, a.GenerateProfilesNow(context.Background()), "db down")
	assert.Zero(t, llm.profileCalls())
	var lastError string
	require.NoError(t, testDB.QueryRow(context.Background(),
		"SELECT last_error FROM taste.profiles WHERE user_id = $1", userID,
	).Scan(&lastError))
	assert.Equal(t, "Your history couldn't be read.", lastError)
}

func TestProfile_RefreshDuringGenerationStaysPending(t *testing.T) {
	llm := newLLM(profileJSON)
	a, c := profileApp(t, sharedmocks.NewMockedAdminAuthService(userID), llm)
	llm.during = func() { assert.NoError(t, refresh(c)) }

	require.NoError(t, a.GenerateProfilesNow(context.Background()))
	p := getProfile(t, c)
	require.NotNil(t, p.Sections)
	assert.True(t, p.Pending, "the mid-run request still gets its own run")
}

func TestProfile_NothingConsumedClearsTheProfile(t *testing.T) {
	llm := newLLM(profileJSON)
	withBook, _ := profileApp(t, sharedmocks.NewMockedAdminAuthService(userID), llm)
	require.NoError(t, withBook.GenerateProfilesNow(context.Background()))
	_, err := testDB.Exec(context.Background(),
		"UPDATE taste.profiles SET pending = true WHERE user_id = $1", userID)
	require.NoError(t, err)

	cfg := testCfg
	cfg.OpenRouterAPIKey = "test-key"
	empty := taste.NewInner(sharedmocks.NewMockedAdminAuthService(userID),
		logging.NewNopLogger(), cfg, testDB, llm, noCatalogs())
	require.NoError(t, empty.GenerateProfilesNow(context.Background()))
	assert.Equal(t, 1, llm.profileCalls(), "no completion for an empty log")

	ts := httptest.NewServer(testhelper.BuildMux(empty))
	t.Cleanup(ts.Close)
	c := tastev1connect.NewTasteServiceClient(
		connect.NewClient(
			connecthttp.NewTransport(
				http.DefaultClient,
				ts.URL,
				connecthttp.WithReadMaxBytes(0),
			),
		),
	)
	p := getProfile(t, c)
	assert.Nil(t, p.Sections, "no profile outlives the data it came from")
	assert.Empty(t, p.GeneratedAt)
	assert.Empty(t, p.Model)
	assert.False(t, p.Pending)
	assert.Equal(t, "There's nothing in your history to write a profile from yet.",
		p.LastError)
}

func TestProfile_JobSkipsUsersWithoutTasteAccess(t *testing.T) {
	llm := newLLM(profileJSON)
	// The mocked non-admin user has no taste access.
	a, _ := profileApp(t, sharedmocks.NewMockedAuthService(userID), llm)
	require.NoError(t, a.GenerateProfilesNow(context.Background()))
	assert.Zero(t, llm.profileCalls())
}

func TestProfile_JobIdleWithoutKey(t *testing.T) {
	llm := newLLM(profileJSON)
	a := taste.NewInner(sharedmocks.NewMockedAdminAuthService(userID),
		logging.NewNopLogger(), testCfg, testDB, llm, noCatalogs())
	require.NoError(t, a.GenerateProfilesNow(context.Background()))
	assert.Zero(t, llm.profileCalls())
}
