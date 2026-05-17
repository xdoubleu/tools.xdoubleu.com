package backlog_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/test"
	"tools.xdoubleu.com/apps/backlog"
	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
)

// seedSteamData imports steam games for userID using the mock client.
// It saves integrations with dummy steam credentials so ImportOwnedGames
// uses the mock factory (the actual keys are ignored by the mock).
func seedSteamData(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	// Save dummy integrations so HasCompletedOnboarding == true and
	// ImportOwnedGames can build a client from the factory.
	err := testApp.SaveIntegrations(
		ctx,
		userID,
		backlog.Integrations{ //nolint:exhaustruct //HardcoverAPIKey not needed for steam seed
			SteamAPIKey: "test-steam-api-key",
			SteamUserID: "76561197960287930",
		},
	)
	require.NoError(t, err)

	_, err = testApp.Services.Steam.ImportOwnedGames(ctx, userID)
	require.NoError(t, err)
}

// TestSteamGameHandler_ValidID seeds the steam library then hits the game
// detail page for the mock game (AppID=1), exercising the full happy path
// of steamGameHandler and the SteamGamePage templ component.
func TestSteamGameHandler_ValidID(t *testing.T) {
	seedSteamData(t)

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/steam/games/1",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

// TestSteamDistributionPage_WithData seeds steam data so that the progress
// service can compute a non-empty distribution, exercising more branches of
// DistributionPage templ component (bucket with actual games vs empty).
func TestSteamDistributionPage_WithData(t *testing.T) {
	seedSteamData(t)

	// The mock returns one game (AppID=1) with achievements; after import its
	// completion rate is computed.  Hit every bucket (0-10) so we exercise
	// both the empty-list and the non-empty-list branches of DistributionPage.
	for i := 0; i <= 10; i++ {
		tReq := test.CreateRequestTester(
			getRoutes(),
			http.MethodGet,
			"/"+testApp.GetName()+"/steam/distribution/"+itoa(i),
		)
		tReq.AddCookie(&accessToken)

		rs := tReq.Do(t)
		assert.Equal(t, http.StatusOK, rs.StatusCode)
	}
}

// itoa converts an int to a string without importing strconv in the test body.
func itoa(n int) string {
	return [...]string{
		"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10",
	}[n]
}

// addTestBookWithStatus adds a book directly via the service layer for an
// isolated userID, so the shared userID's library is not polluted and
// BooksLibraryPage tests get predictable data.
func addTestBookWithStatus(
	t *testing.T,
	title string,
	status string,
	tags []string,
) *models.UserBook {
	t.Helper()
	desc := "A description."
	ext := hardcover.ExternalBook{ //nolint:exhaustruct //optional fields
		Provider:    "manual",
		ProviderID:  "test-lib-" + title,
		Title:       title,
		Authors:     []string{"Author"},
		Description: &desc,
	}
	ub, err := testApp.Services.Books.AddToLibrary(
		context.Background(),
		userID,
		ext,
		status,
		tags,
	)
	require.NoError(t, err)
	require.NotNil(t, ub)
	return ub
}

// TestBooksLibraryPage_WithData populates the library with books of every
// status and with custom/tag shelves, then fetches the library page.
// This exercises the non-empty conditional branches in BooksLibraryPage.
func TestBooksLibraryPage_WithData(t *testing.T) {
	addTestBookWithStatus(t, "LibraryReadingBook", models.StatusReading, []string{})
	addTestBookWithStatus(t, "LibraryToReadBook", models.StatusToRead, []string{})
	addTestBookWithStatus(t, "LibraryReadBook", models.StatusRead, []string{})
	addTestBookWithStatus(t, "LibraryTagBook", models.StatusToRead, []string{"sci-fi"})

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/books/library",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

// TestBooksProgressPage_WithData marks a book as read so that the progress
// service returns non-nil labels/values, exercising the populated branches
// of BooksProgressPage and booksProgressScript.
func TestBooksProgressPage_WithData(t *testing.T) {
	ub := addTestBook(t, "ProgressPageBook")

	// Mark the book as read so a progress entry is recorded.
	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodPost,
		"/"+testApp.GetName()+"/books/"+ub.BookID.String()+"/status",
	)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(struct {
		Status string `schema:"status"`
		Rating string `schema:"rating"`
	}{Status: models.StatusRead, Rating: "4"})
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)

	// Now fetch the progress page — labels/values will be non-empty.
	tReq2 := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/books/progress",
	)
	tReq2.AddCookie(&accessToken)
	rs2 := tReq2.Do(t)
	assert.Equal(t, http.StatusOK, rs2.StatusCode)
}

// TestBooksLibraryPage_FavouriteAndOwnership adds a book with special tags so
// the bookCard templ branches for own-physical, own-digital and favourite are
// exercised.
func TestBooksLibraryPage_FavouriteAndOwnership(t *testing.T) {
	addTestBookWithStatus(
		t,
		"FavouriteOwnedBook",
		models.StatusRead,
		[]string{
			models.TagFavourite,
			models.TagOwnPhysical,
			models.TagOwnDigital,
		},
	)

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/books/library",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

// TestSteamPageWithData seeds steam data then fetches the steam overview,
// exercising the non-empty InProgress/Completed branches of SteamPage.
func TestSteamPageWithData(t *testing.T) {
	seedSteamData(t)

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/steam",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

// TestSteamPage_WithBacklogAndInProgress inserts a "not-started" game
// (completion_rate=0, has unachieved achievement) and an in-progress game
// (completion_rate=50, has unachieved achievement) directly into the DB, then
// fetches the steam overview page. This exercises the loop bodies of
// GetBacklog and GetInProgress, plus the non-empty branches in SteamPage.
func TestSteamPage_WithBacklogAndInProgress(t *testing.T) {
	ctx := context.Background()

	seedSteamData(t)

	_, err := testDB.Exec(ctx, `
		INSERT INTO backlog.steam_games
			(id, user_id, name, completion_rate, contribution, playtime_forever)
		VALUES (9901, $1, 'Backlog Game', '0.00', '0.0000', 0)
		ON CONFLICT (id, user_id) DO NOTHING
	`, userID)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, `
		INSERT INTO backlog.steam_achievements (name, user_id, game_id, achieved)
		VALUES ('BACKLOG_ACH', $1, 9901, false)
		ON CONFLICT (name, user_id, game_id) DO NOTHING
	`, userID)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, `
		INSERT INTO backlog.steam_games
			(id, user_id, name, completion_rate, contribution, playtime_forever)
		VALUES (9902, $1, 'InProgress Game', '50.00', '0.0000', 0)
		ON CONFLICT (id, user_id) DO NOTHING
	`, userID)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, `
		INSERT INTO backlog.steam_achievements (name, user_id, game_id, achieved)
		VALUES ('IP_ACH', $1, 9902, false)
		ON CONFLICT (name, user_id, game_id) DO NOTHING
	`, userID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.steam_games WHERE id IN (9901, 9902) AND user_id = $1`,
			userID,
		)
	})

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/steam",
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

// TestSteamCompletionRate_NotFound covers the ErrResourceNotFound → "0.00"
// branch of GetCurrentSteamCompletionRate using an isolated user.
func TestSteamCompletionRate_NotFound(t *testing.T) {
	const isolatedUser = "steam-rate-notfound-user"
	rate, err := testApp.Services.Progress.GetCurrentSteamCompletionRate(
		context.Background(), isolatedUser,
	)
	require.NoError(t, err)
	assert.Equal(t, "0.00", rate)
}

// TestSteamCompletionRate_WithRecord saves a steam progress entry then reads it
// back, covering the return value branch of GetCurrentSteamCompletionRate.
func TestSteamCompletionRate_WithRecord(t *testing.T) {
	ctx := context.Background()
	const isolatedUser = "steam-rate-record-user"
	today := time.Now().UTC().Format(models.ProgressDateFormat)

	err := testApp.Services.Progress.Save(
		ctx, models.SteamTypeID, isolatedUser, []string{today}, []string{"55.00"},
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.progress WHERE user_id = $1 AND type_id = $2`,
			isolatedUser, models.SteamTypeID,
		)
	})

	rate, err := testApp.Services.Progress.GetCurrentSteamCompletionRate(
		ctx, isolatedUser,
	)
	require.NoError(t, err)
	assert.Equal(t, "55.00", rate)
}

// mockEmptyAchievementsSteamClient is a steam client whose GetPlayerAchievements
// returns an empty achievement list, triggering the UpsertAchievementSchemas path.
type mockEmptyAchievementsSteamClient struct{}

func (mockEmptyAchievementsSteamClient) GetOwnedGames(
	_ context.Context,
	_ string,
) (*steam.OwnedGamesResponse, error) {
	return &steam.OwnedGamesResponse{
		Response: steam.OwnedGamesResponseData{
			GameCount: 1,
			Games: []steam.Game{
				{ //nolint:exhaustruct //only required fields
					AppID:                    9999,
					Name:                     "no-achievements game",
					HasCommunityVisibleStats: true,
				},
			},
		},
	}, nil
}

func (mockEmptyAchievementsSteamClient) GetPlayerAchievements(
	_ context.Context,
	steamID string,
	_ int,
) (*steam.AchievementsResponse, error) {
	return &steam.AchievementsResponse{
		PlayerStats: steam.PlayerStats{
			Success:      true,
			SteamID:      steamID,
			GameName:     "no-achievements game",
			Achievements: []steam.Achievement{},
		},
	}, nil
}

func (mockEmptyAchievementsSteamClient) GetSchemaForGame(
	_ context.Context,
	_ int,
) (*steam.GetSchemaForGameResponse, error) {
	return &steam.GetSchemaForGameResponse{
		Game: steam.GameSchema{ //nolint:exhaustruct //only required fields
			AvailableGameStats: steam.AvailableGameStats{
				Achievements: []steam.AchievementSchema{
					{ //nolint:exhaustruct //only required fields
						Name:        "SCHEMA_ACH",
						DisplayName: "Schema Achievement",
					},
				},
			},
		},
	}, nil
}

func (mockEmptyAchievementsSteamClient) GetGlobalAchievementPercentagesForApp(
	_ context.Context,
	_ int,
) (*steam.GlobalAchievementPercentagesResponse, error) {
	//nolint:exhaustruct //anonymous inner struct via field assignment
	resp := steam.GlobalAchievementPercentagesResponse{}
	resp.AchievementPercentages.Achievements = []steam.GlobalAchievementPercent{}
	return &resp, nil
}

// TestUpsertAchievementSchemas exercises the UpsertAchievementSchemas repository
// method by importing games with a steam client that returns no player achievements.
func TestUpsertAchievementSchemas(t *testing.T) {
	ctx := context.Background()
	const isolatedUserID = "upsert-schema-test-user-id"

	app2 := backlog.NewInner(
		ctx,
		sharedmocks.NewMockedAuthService(isolatedUserID),
		testApp.Logger,
		testCfg,
		testDB,
		backlog.Clients{
			SteamFactory: func(_ string) steam.Client {
				return mockEmptyAchievementsSteamClient{}
			},
			HardcoverFactory: func(_ string) hardcover.Client {
				return nil
			},
		},
	)

	err := app2.SaveIntegrations(
		ctx,
		isolatedUserID,
		backlog.Integrations{ //nolint:exhaustruct //only steam needed
			SteamAPIKey: "test-key",
			SteamUserID: "76561197960287930",
		},
	)
	require.NoError(t, err)

	_, err = app2.Services.Steam.ImportOwnedGames(ctx, isolatedUserID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.steam_games WHERE user_id = $1`,
			isolatedUserID,
		)
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.integrations WHERE user_id = $1`,
			isolatedUserID,
		)
	})
}

// TestSteamGameHandler_SortComparator inserts a game with four achievements
// whose global_percent values cover all four branches of the sort.Slice
// comparator in steamGameHandler:
//   - both nil  → compare by display name
//   - pi nil    → return false (nil goes after non-nil)
//   - pj nil    → return true  (non-nil goes before nil)
//   - both set  → compare by value descending
func TestSteamGameHandler_SortComparator(t *testing.T) {
	ctx := context.Background()
	const sortGameID = 9903

	seedSteamData(t)

	_, err := testDB.Exec(ctx, `
		INSERT INTO backlog.steam_games
			(id, user_id, name, completion_rate, contribution, playtime_forever)
		VALUES ($1, $2, 'Sort Comparator Game', '50.00', '0.0000', 0)
		ON CONFLICT (id, user_id) DO NOTHING
	`, sortGameID, userID)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, `
		INSERT INTO backlog.steam_achievements
			(name, display_name, description, icon_url, user_id, game_id,
			 achieved, unlock_time, global_percent)
		VALUES
			('ACH_HIGH',  'Zeta Achievement',  '', '', $1, $2, false, NULL, 80.0),
			('ACH_LOW',   'Alpha Achievement', '', '', $1, $2, false, NULL, 20.0),
			('ACH_NIL_A', 'Gamma Achievement', '', '', $1, $2, false, NULL, NULL),
			('ACH_NIL_B', 'Delta Achievement', '', '', $1, $2, false, NULL, NULL)
		ON CONFLICT (name, user_id, game_id) DO NOTHING
	`, userID, sortGameID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.steam_games WHERE id = $1 AND user_id = $2`,
			sortGameID, userID,
		)
	})

	tReq := test.CreateRequestTester(
		getRoutes(),
		http.MethodGet,
		"/"+testApp.GetName()+"/steam/games/"+strconv.Itoa(sortGameID),
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}
