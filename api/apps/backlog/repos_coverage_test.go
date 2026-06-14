package backlog_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/backlog"
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
	"tools.xdoubleu.com/apps/backlog/pkg/objectstore"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
	backlogv1 "tools.xdoubleu.com/gen/backlog/v1"
	backlogv1connect "tools.xdoubleu.com/gen/backlog/v1/backlogv1connect"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

// TestFindBookByTitleAndAuthor_NotFound exercises the repository method when no
// matching book exists — covers the 0-coverage path.
func TestFindBookByTitleAndAuthor_NotFound(t *testing.T) {
	book, err := testApp.Repositories.Books.FindBookByTitleAndAuthor(
		context.Background(),
		"nonexistent-title-xyz",
		"nonexistent-author-xyz",
	)
	require.Error(t, err)
	assert.Nil(t, book)
}

// TestFindBookByTitleAndAuthor_Found adds a book and then looks it up by title/author.
func TestFindBookByTitleAndAuthor_Found(t *testing.T) {
	ub := addTestBook(t, "FindByTitleBook")
	require.NotNil(t, ub)

	book, err := testApp.Repositories.Books.FindBookByTitleAndAuthor(
		context.Background(),
		"FindByTitleBook",
		"Test Author",
	)
	require.NoError(t, err)
	assert.NotNil(t, book)
	assert.Equal(t, "FindByTitleBook", book.Title)
}

// TestFindByExternalRef_NotFound exercises FindByExternalRef when the ref is absent.
func TestFindByExternalRef_NotFound(t *testing.T) {
	book, err := testApp.Repositories.Books.FindByExternalRef(
		context.Background(),
		"manual",
		"nonexistent-provider-id-xyz",
	)
	require.Error(t, err)
	assert.Nil(t, book)
}

// TestFindByExternalRef_Found adds a book via AddToLibrary (which stores an
// external_ref) then retrieves it via FindByExternalRef.
func TestFindByExternalRef_Found(t *testing.T) {
	isbn := "9780000001234"
	providerID := "extref-coverage-test-id"
	cover := "https://example.com/cover.jpg"
	ext := hardcover.ExternalBook{ //nolint:exhaustruct //optional fields not needed
		Provider:   "manual",
		ProviderID: providerID,
		Title:      "FindByExternalRefBook",
		Authors:    []string{"External Author"},
		ISBN13:     &isbn,
		CoverURL:   &cover,
	}
	_, err := testApp.Services.Books.AddToLibrary(
		context.Background(), userID, ext, "to-read", []string{},
	)
	require.NoError(t, err)

	book, err := testApp.Repositories.Books.FindByExternalRef(
		context.Background(),
		"manual",
		providerID,
	)
	require.NoError(t, err)
	assert.NotNil(t, book)
}

// TestIntegrationsExists_False covers Exists when no record is stored for user.
func TestIntegrationsExists_False(t *testing.T) {
	exists, err := testApp.Repositories.Integrations.Exists(
		context.Background(),
		"integrations-exists-no-record-user",
	)
	require.NoError(t, err)
	assert.False(t, exists)
}

// TestIntegrationsExists_True saves integrations then checks Exists returns true.
func TestIntegrationsExists_True(t *testing.T) {
	const isolatedUser = "integrations-exists-true-user"
	err := testApp.SaveIntegrations(
		context.Background(),
		isolatedUser,
		backlog.Integrations{}, //nolint:exhaustruct //fields default to ""
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.user_integrations WHERE user_id = $1`, isolatedUser)
	})

	exists, err := testApp.Repositories.Integrations.Exists(
		context.Background(),
		isolatedUser,
	)
	require.NoError(t, err)
	assert.True(t, exists)
}

// zeroAchievementsSteamClient returns a game with one unachieved achievement so
// the game lands in GetBacklog (completion_rate = 0, achievements exist).
type zeroAchievementsSteamClient struct{}

func (zeroAchievementsSteamClient) GetOwnedGames(
	_ context.Context,
	_ string,
) (*steam.OwnedGamesResponse, error) {
	return &steam.OwnedGamesResponse{
		Response: steam.OwnedGamesResponseData{
			GameCount: 1,
			Games: []steam.Game{
				{ //nolint:exhaustruct //only required fields
					AppID:                    8001,
					Name:                     "zero-completion game",
					HasCommunityVisibleStats: true,
				},
			},
		},
	}, nil
}

func (zeroAchievementsSteamClient) GetPlayerAchievements(
	_ context.Context,
	steamID string,
	_ int,
) (*steam.AchievementsResponse, error) {
	return &steam.AchievementsResponse{
		PlayerStats: steam.PlayerStats{
			Success:  true,
			SteamID:  steamID,
			GameName: "zero-completion game",
			Achievements: []steam.Achievement{
				{
					APIName:     "ZERO_ACH",
					Achieved:    0,
					UnlockTime:  0,
					Name:        "Unearned",
					Description: "",
				},
			},
		},
	}, nil
}

func (zeroAchievementsSteamClient) GetSchemaForGame(
	_ context.Context,
	_ int,
) (*steam.GetSchemaForGameResponse, error) {
	//nolint:exhaustruct //skip
	return &steam.GetSchemaForGameResponse{}, nil
}

func (zeroAchievementsSteamClient) GetGlobalAchievementPercentagesForApp(
	_ context.Context,
	_ int,
) (*steam.GlobalAchievementPercentagesResponse, error) {
	//nolint:exhaustruct //anonymous inner struct via field assignment
	resp := steam.GlobalAchievementPercentagesResponse{}
	resp.AchievementPercentages.Achievements = []steam.GlobalAchievementPercent{
		{Name: "ZERO_ACH", Percent: "50.0"},
	}
	return &resp, nil
}

// TestConnectGetSteam_WithBacklogAndInProgress covers the GetBacklog and
// GetInProgress row-loop bodies by seeding two isolated users: one whose game
// has 0% completion (GetBacklog) and one with 50% (GetInProgress via
// twoAchievementsMock).
func TestConnectGetSteam_WithBacklogAndInProgress(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const backlogUser = "steam-getbacklog-coverage-user"
	const inProgressUser = "steam-getinprogress-coverage-user"

	setupSteamUser := func(
		isolatedUser string,
		factory func(string) steam.Client,
	) *backlog.Backlog {
		app2 := backlog.NewInner(
			ctx,
			sharedmocks.NewMockedAuthService(isolatedUser),
			testApp.Logger,
			testCfg,
			testDB,
			backlog.Clients{
				SteamFactory:     factory,
				HardcoverFactory: func(_ string) hardcover.Client { return nil },
				ObjectStore:      objectstore.NewFake(),
				KoboStoreBaseURL: "",
				PublicAPIBaseURL: "",
			},
		)
		err := app2.SaveIntegrations(
			ctx,
			isolatedUser,
			backlog.Integrations{
				SteamUserID: "76561197960287930",
			},
		)
		require.NoError(t, err)
		err = app2.Services.Steam.SyncUser(ctx, isolatedUser)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = testDB.Exec(context.Background(),
				`DELETE FROM backlog.steam_games WHERE user_id = $1`, isolatedUser)
			_, _ = testDB.Exec(context.Background(),
				`DELETE FROM backlog.integrations WHERE user_id = $1`, isolatedUser)
			_, _ = testDB.Exec(
				context.Background(),
				`DELETE FROM backlog.user_integrations WHERE user_id = $1`,
				isolatedUser,
			)
		})
		return app2
	}

	backlogApp := setupSteamUser(backlogUser, func(_ string) steam.Client {
		return zeroAchievementsSteamClient{}
	})
	inProgressApp := setupSteamUser(inProgressUser, func(_ string) steam.Client {
		return twoAchievementsMock{}
	})

	for _, app2 := range []*backlog.Backlog{backlogApp, inProgressApp} {
		ts := httptest.NewServer(testhelper.BuildMux(app2))
		t.Cleanup(ts.Close)
		client := backlogv1connect.NewGamesServiceClient(
			http.DefaultClient,
			ts.URL,
			connect.WithHTTPGet(),
		)

		req := connect.NewRequest(&backlogv1.GetSteamRequest{})
		req.Header().Set("Cookie", accessToken.String())

		resp, err := client.GetSteam(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, resp.Msg.Steam)
	}
}
