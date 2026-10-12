package taste_test

import (
	"context"
	"errors"
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
	a := taste.NewInner(authService, logging.NewNopLogger(), cfg, testDB, llm,
		userSource{name: "books", events: []consumption.Event{book}, err: nil},
	)
	ts := httptest.NewServer(testhelper.BuildMux(a))
	t.Cleanup(ts.Close)
	return a, tastev1connect.NewTasteServiceClient(http.DefaultClient, ts.URL)
}

func getProfile(
	t *testing.T,
	c tastev1connect.TasteServiceClient,
) *tastev1.GetProfileResponse {
	t.Helper()
	resp, err := c.GetProfile(context.Background(), connect.NewRequest(
		&tastev1.GetProfileRequest{},
	))
	require.NoError(t, err)
	return resp.Msg
}

func refresh(c tastev1connect.TasteServiceClient) error {
	_, err := c.RefreshProfile(context.Background(), connect.NewRequest(
		&tastev1.RefreshProfileRequest{},
	))
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
	llm := &fakeLLM{content: profileJSON}
	a, c := profileApp(t, sharedmocks.NewMockedAdminAuthService(userID), llm)

	p := getProfile(t, c)
	assert.True(t, p.Configured)
	assert.Nil(t, p.Sections)
	assert.False(t, p.Pending)
	assert.Empty(t, p.NextRefreshAt)

	require.NoError(t, refresh(c))
	assert.Equal(t, connect.CodeResourceExhausted, connect.CodeOf(refresh(c)))
	p = getProfile(t, c)
	assert.True(t, p.Pending)
	next, err := time.Parse(time.RFC3339, p.NextRefreshAt)
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

	require.Len(t, llm.requests, 1)
	req := llm.requests[0]
	assert.Equal(t, testCfg.TasteGeneratorModel, req.Model)
	assert.Equal(t, "taste_profile", req.SchemaName)
	assert.Nil(t, req.WebSearch)
	require.Len(t, req.Messages, 2)
	assert.Contains(t, req.Messages[1].Content, "Dune — Frank Herbert [sci-fi]")

	// Fresh, so the next run leaves it alone.
	require.NoError(t, a.GenerateProfilesNow(context.Background()))
	assert.Len(t, llm.requests, 1)
}

func TestProfile_FailureKeepsLastProfile(t *testing.T) {
	llm := &fakeLLM{content: profileJSON}
	a, c := profileApp(t, sharedmocks.NewMockedAdminAuthService(userID), llm)
	require.NoError(t, a.GenerateProfilesNow(context.Background()))

	_, err := testDB.Exec(context.Background(), `
		UPDATE taste.profiles SET pending = true, requested_at = NULL
		WHERE user_id = $1`, userID)
	require.NoError(t, err)
	llm.err = errors.New("model overloaded")
	require.ErrorContains(t, a.GenerateProfilesNow(context.Background()), "model overloaded")

	p := getProfile(t, c)
	assert.Equal(t, "model overloaded", p.LastError)
	assert.False(t, p.Pending)
	require.NotNil(t, p.Sections, "the previous profile stays")

	// Failed attempts count too, so it isn't retried hourly.
	require.NoError(t, a.GenerateProfilesNow(context.Background()))
	assert.Len(t, llm.requests, 2)
}

func TestProfile_InvalidAnswers(t *testing.T) {
	for _, content := range []string{`not json`, `{"summary": "  "}`} {
		llm := &fakeLLM{content: content}
		a, c := profileApp(t, sharedmocks.NewMockedAdminAuthService(userID), llm)
		require.Error(t, a.GenerateProfilesNow(context.Background()))
		p := getProfile(t, c)
		assert.NotEmpty(t, p.LastError)
		assert.Nil(t, p.Sections)
	}
}

func TestProfile_NothingConsumedIsNoFailure(t *testing.T) {
	_, err := testDB.Exec(context.Background(),
		"DELETE FROM taste.profiles WHERE user_id = $1", userID)
	require.NoError(t, err)
	cfg := testCfg
	cfg.OpenRouterAPIKey = "test-key"
	llm := &fakeLLM{content: profileJSON}
	a := taste.NewInner(sharedmocks.NewMockedAdminAuthService(userID),
		logging.NewNopLogger(), cfg, testDB, llm)

	require.NoError(t, a.GenerateProfilesNow(context.Background()))
	assert.Empty(t, llm.requests)
	var lastError string
	require.NoError(t, testDB.QueryRow(context.Background(),
		"SELECT last_error FROM taste.profiles WHERE user_id = $1", userID,
	).Scan(&lastError))
	assert.Equal(t, "Nothing consumed yet.", lastError)
}

func TestProfile_JobSkipsUsersWithoutTasteAccess(t *testing.T) {
	llm := &fakeLLM{content: profileJSON}
	// The mocked non-admin user has no taste access.
	a, _ := profileApp(t, sharedmocks.NewMockedAuthService(userID), llm)
	require.NoError(t, a.GenerateProfilesNow(context.Background()))
	assert.Empty(t, llm.requests)
}

func TestProfile_JobIdleWithoutKey(t *testing.T) {
	llm := &fakeLLM{content: profileJSON}
	a := taste.NewInner(sharedmocks.NewMockedAdminAuthService(userID),
		logging.NewNopLogger(), testCfg, testDB, llm)
	require.NoError(t, a.GenerateProfilesNow(context.Background()))
	assert.Empty(t, llm.requests)
}
