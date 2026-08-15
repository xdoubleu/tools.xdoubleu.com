package dashboard_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/games"
	dashboardv1 "tools.xdoubleu.com/gen/dashboard/v1"
	"tools.xdoubleu.com/gen/dashboard/v1/dashboardv1connect"
	sharedmodels "tools.xdoubleu.com/internal/models"
	sharedrepos "tools.xdoubleu.com/internal/repositories"
)

// publicGamesUserID/publicReadingUserID own the data behind the two
// dashboards' tests — distinct so seeding one dashboard's data never
// collides with the other's, and distinct from cmd/api's own
// DashboardService tests, which use their own testUserID.
const publicGamesUserID = "eeeeeeee-1111-2222-3333-444444444444"
const publicReadingUserID = "dddddddd-1111-2222-3333-444444444444"

const publicGamesToken = "test-games-dashboard-token"
const publicReadingToken = "test-reading-dashboard-token"
const publicDisplayName = "Public Dashboard Owner"

// ensureProfileShare mirrors cmd/api/migrations/00001_init.sql,
// 00007_profile_shares_per_app.sql, and 00014_dashboard_shares_reading.sql
// so these tests can run before the cmd/api package has applied the global
// migrations, then links the two dashboard tokens above to their owners.
func ensureProfileShare(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	stmts := []string{
		"CREATE SCHEMA IF NOT EXISTS global",
		`CREATE TABLE IF NOT EXISTS global.app_users (
			id           TEXT PRIMARY KEY,
			email        TEXT NOT NULL,
			last_seen    TIMESTAMPTZ NOT NULL DEFAULT now(),
			role         TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin','user')),
			display_name TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS global.profile_shares (
			user_id TEXT NOT NULL,
			app TEXT NOT NULL CHECK (app IN ('reading', 'games')),
			token TEXT UNIQUE NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (user_id, app)
		)`,
	}
	for _, stmt := range stmts {
		_, err := testDB.Exec(ctx, stmt)
		require.NoError(t, err)
	}

	for _, userID := range []string{publicGamesUserID, publicReadingUserID} {
		_, err := testDB.Exec(ctx, `
			INSERT INTO global.app_users (id, email, display_name)
			VALUES ($1, $1 || '@example.com', $2)
			ON CONFLICT (id) DO UPDATE SET display_name = EXCLUDED.display_name
		`, userID, publicDisplayName)
		require.NoError(t, err)
	}

	repo := sharedrepos.NewProfileSharesRepository(testDB)
	_, err := repo.Upsert(
		ctx,
		publicGamesUserID,
		sharedmodels.DashboardKindGames,
		publicGamesToken,
	)
	require.NoError(t, err)
	_, err = repo.Upsert(
		ctx, publicReadingUserID, sharedmodels.DashboardKindReading, publicReadingToken,
	)
	require.NoError(t, err)
}

// seedPublicSteamData syncs the fake Steam library for publicGamesUserID.
func seedPublicSteamData(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	require.NoError(
		t,
		testGames.SaveIntegrations(ctx, publicGamesUserID, games.Integrations{
			SteamUserID: "76561197960287930",
		}),
	)
	require.NoError(t, testGames.Services.Steam.SyncUser(ctx, publicGamesUserID))
}

// seedPublicBook inserts a wishlist book directly via SQL — books'
// AddToLibrary takes an apps/books/internal/services type this package
// cannot import, so seeding goes straight to the schema instead.
func seedPublicBook(t *testing.T, title string) string {
	t.Helper()
	ctx := context.Background()

	var bookID string
	require.NoError(t, testDB.QueryRow(ctx, `
		INSERT INTO books.books (title, authors) VALUES ($1, '{Public Author}')
		RETURNING id
	`, title).Scan(&bookID))

	_, err := testDB.Exec(ctx, `
		INSERT INTO books.user_books (user_id, book_id, status, tags)
		VALUES ($1, $2, 'to-read', '{favourite}')
		ON CONFLICT (user_id, book_id) DO NOTHING
	`, publicReadingUserID, bookID)
	require.NoError(t, err)
	return bookID
}

// seedPublicFeedItem inserts an unread feed item directly via SQL for
// publicReadingUserID.
func seedPublicFeedItem(t *testing.T, title string) {
	t.Helper()
	ctx := context.Background()

	var feedID string
	require.NoError(t, testDB.QueryRow(ctx, `
		INSERT INTO feeds.feeds (user_id, url, title) VALUES ($1, $2, 'Test Feed')
		ON CONFLICT (user_id, url) DO UPDATE SET title = EXCLUDED.title
		RETURNING id
	`, publicReadingUserID, "https://example.com/feed-"+title).Scan(&feedID))

	_, err := testDB.Exec(ctx, `
		INSERT INTO feeds.items (feed_id, guid, title, source_url, read_at)
		VALUES ($1, $2, $3, 'https://example.com/item', NULL)
		ON CONFLICT (feed_id, guid) DO UPDATE SET read_at = NULL
	`, feedID, title, title)
	require.NoError(t, err)
}

// newPublicClient returns a client with NO auth cookie — dashboard's public
// RPCs must work without a session.
func newPublicClient(
	t *testing.T,
) dashboardv1connect.PublicGamesDashboardServiceClient {
	t.Helper()
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	return dashboardv1connect.NewPublicGamesDashboardServiceClient(
		http.DefaultClient,
		ts.URL,
	)
}

func newPublicReadingClient(
	t *testing.T,
) dashboardv1connect.PublicReadingDashboardServiceClient {
	t.Helper()
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	return dashboardv1connect.NewPublicReadingDashboardServiceClient(
		http.DefaultClient,
		ts.URL,
	)
}

func TestGetSharedSteam_Success(t *testing.T) {
	ensureProfileShare(t)
	seedPublicSteamData(t)

	client := newPublicClient(t)
	resp, err := client.GetSharedSteam(
		context.Background(),
		connect.NewRequest(&dashboardv1.GetSharedSteamRequest{Token: publicGamesToken}),
	)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Steam)
	assert.NotEmpty(t, resp.Msg.LastSyncedAt)
	assert.Equal(t, publicDisplayName, resp.Msg.DisplayName)
	total := len(resp.Msg.Steam.NotStarted) +
		len(resp.Msg.Steam.InProgress) +
		len(resp.Msg.Steam.Completed)
	assert.Positive(t, total, "seeded library should contain games")
}

func TestGetSharedSteam_InvalidDateRange(t *testing.T) {
	ensureProfileShare(t)
	seedPublicSteamData(t)

	// Malformed date strings fall back to the default one-year window
	// (parseDateRangeFromStrings) instead of erroring.
	client := newPublicClient(t)
	resp, err := client.GetSharedSteam(
		context.Background(),
		connect.NewRequest(&dashboardv1.GetSharedSteamRequest{
			Token:     publicGamesToken,
			DateStart: "not-a-date",
			DateEnd:   "also-not-a-date",
		}),
	)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Steam)
}

func TestGetSharedSteam_UnknownToken(t *testing.T) {
	ensureProfileShare(t)

	client := newPublicClient(t)
	_, err := client.GetSharedSteam(
		context.Background(),
		connect.NewRequest(
			&dashboardv1.GetSharedSteamRequest{Token: "definitely-not-a-token"},
		),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetSharedSteam_EmptyToken(t *testing.T) {
	ensureProfileShare(t)

	client := newPublicClient(t)
	_, err := client.GetSharedSteam(
		context.Background(),
		connect.NewRequest(&dashboardv1.GetSharedSteamRequest{}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetSharedSteam_WrongKindToken(t *testing.T) {
	ensureProfileShare(t)

	// The reading dashboard's token must not resolve on the games endpoint,
	// even though both may belong to the same conceptual owner.
	client := newPublicClient(t)
	_, err := client.GetSharedSteam(
		context.Background(),
		connect.NewRequest(
			&dashboardv1.GetSharedSteamRequest{Token: publicReadingToken},
		),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetSharedSteamGame_Success(t *testing.T) {
	ensureProfileShare(t)
	seedPublicSteamData(t)

	client := newPublicClient(t)
	resp, err := client.GetSharedSteamGame(
		context.Background(),
		connect.NewRequest(&dashboardv1.GetSharedSteamGameRequest{
			Token: publicGamesToken, GameId: 1,
		}),
	)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Data)
	require.NotNil(t, resp.Msg.Data.Game)
}

func TestGetSharedSteamGame_UnknownGame(t *testing.T) {
	ensureProfileShare(t)
	seedPublicSteamData(t)

	client := newPublicClient(t)
	_, err := client.GetSharedSteamGame(
		context.Background(),
		connect.NewRequest(&dashboardv1.GetSharedSteamGameRequest{
			Token: publicGamesToken, GameId: 999999,
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetSharedSteamGame_UnknownToken(t *testing.T) {
	ensureProfileShare(t)

	client := newPublicClient(t)
	_, err := client.GetSharedSteamGame(
		context.Background(),
		connect.NewRequest(&dashboardv1.GetSharedSteamGameRequest{
			Token: "definitely-not-a-token", GameId: 1,
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetSharedRecentlyActiveGames_Success(t *testing.T) {
	ensureProfileShare(t)
	seedPublicSteamData(t)

	client := newPublicClient(t)
	resp, err := client.GetSharedRecentlyActiveGames(
		context.Background(),
		connect.NewRequest(&dashboardv1.GetSharedRecentlyActiveGamesRequest{
			Token: publicGamesToken,
		}),
	)
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg.Games)
}

func TestGetSharedRecentlyActiveGames_UnknownToken(t *testing.T) {
	ensureProfileShare(t)

	client := newPublicClient(t)
	_, err := client.GetSharedRecentlyActiveGames(
		context.Background(),
		connect.NewRequest(&dashboardv1.GetSharedRecentlyActiveGamesRequest{
			Token: "definitely-not-a-token",
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetSharedLibrary_Success(t *testing.T) {
	ensureProfileShare(t)
	bookID := seedPublicBook(t, "Dashboard Test Book")

	client := newPublicReadingClient(t)
	resp, err := client.GetSharedLibrary(
		context.Background(),
		connect.NewRequest(
			&dashboardv1.GetSharedLibraryRequest{Token: publicReadingToken},
		),
	)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Library)
	require.NotEmpty(t, resp.Msg.Library.Wishlist)
	assert.Equal(t, publicDisplayName, resp.Msg.DisplayName)

	var found bool
	for _, ub := range resp.Msg.Library.Wishlist {
		if ub.BookId == bookID {
			found = true
		}
	}
	assert.True(t, found, "seeded book should be in the shared wishlist")
}

func TestGetSharedLibrary_UnknownToken(t *testing.T) {
	ensureProfileShare(t)

	client := newPublicReadingClient(t)
	_, err := client.GetSharedLibrary(
		context.Background(),
		connect.NewRequest(
			&dashboardv1.GetSharedLibraryRequest{Token: "definitely-not-a-token"},
		),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetSharedLibrary_WrongKindToken(t *testing.T) {
	ensureProfileShare(t)

	client := newPublicReadingClient(t)
	_, err := client.GetSharedLibrary(
		context.Background(),
		connect.NewRequest(
			&dashboardv1.GetSharedLibraryRequest{Token: publicGamesToken},
		),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetSharedBooksProgress_Success(t *testing.T) {
	ensureProfileShare(t)

	client := newPublicReadingClient(t)
	resp, err := client.GetSharedBooksProgress(
		context.Background(),
		connect.NewRequest(&dashboardv1.GetSharedBooksProgressRequest{
			Token: publicReadingToken,
		}),
	)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Progress)
	assert.NotEmpty(t, resp.Msg.Progress.DateStart)
	assert.NotEmpty(t, resp.Msg.Progress.DateEnd)
}

func TestGetSharedBooksProgress_UnknownToken(t *testing.T) {
	ensureProfileShare(t)

	client := newPublicReadingClient(t)
	_, err := client.GetSharedBooksProgress(
		context.Background(),
		connect.NewRequest(&dashboardv1.GetSharedBooksProgressRequest{
			Token: "definitely-not-a-token",
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetSharedFeedsSummary_Success(t *testing.T) {
	ensureProfileShare(t)
	seedPublicFeedItem(t, "Dashboard Test Feed Item")

	client := newPublicReadingClient(t)
	resp, err := client.GetSharedFeedsSummary(
		context.Background(),
		connect.NewRequest(&dashboardv1.GetSharedFeedsSummaryRequest{
			Token: publicReadingToken,
		}),
	)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Summary)
	assert.Positive(t, resp.Msg.Summary.UnreadCount)

	var found bool
	for _, item := range resp.Msg.Summary.Items {
		if item.Title == "Dashboard Test Feed Item" {
			found = true
		}
	}
	assert.True(t, found, "seeded item should be in the feeds summary")
}

func TestGetSharedFeedsSummary_UnknownToken(t *testing.T) {
	ensureProfileShare(t)

	client := newPublicReadingClient(t)
	_, err := client.GetSharedFeedsSummary(
		context.Background(),
		connect.NewRequest(&dashboardv1.GetSharedFeedsSummaryRequest{
			Token: "definitely-not-a-token",
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
