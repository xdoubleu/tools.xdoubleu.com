package backlog_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/backlog/internal/models"
	backlogv1 "tools.xdoubleu.com/gen/backlog/v1"
	backlogv1connect "tools.xdoubleu.com/gen/backlog/v1/backlogv1connect"
)

func newBooksTestClient(t *testing.T) backlogv1connect.BooksServiceClient {
	t.Helper()
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	return backlogv1connect.NewBooksServiceClient(
		http.DefaultClient,
		ts.URL,
		connect.WithHTTPGet(),
	)
}

func newGamesTestClient(t *testing.T) backlogv1connect.GamesServiceClient {
	t.Helper()
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	return backlogv1connect.NewGamesServiceClient(
		http.DefaultClient,
		ts.URL,
		connect.WithHTTPGet(),
	)
}

func TestConnectGetSummary(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.GetSummaryRequest{})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetSummary(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg.Summary)
	assert.GreaterOrEqual(t, resp.Msg.Summary.BooksCount, int32(0))
	assert.GreaterOrEqual(t, resp.Msg.Summary.SteamCount, int32(0))
}

func TestConnectGetUserSummary(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.GetUserSummaryRequest{
		UserId: userID,
	})

	resp, err := client.GetUserSummary(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg.Summary)
	assert.GreaterOrEqual(t, resp.Msg.Summary.BooksCount, int32(0))
	assert.GreaterOrEqual(t, resp.Msg.Summary.SteamCount, int32(0))
}

func TestConnectGetLibrary(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.GetLibraryRequest{})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetLibrary(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg.Library)
	// Library may have books from previous tests, just check it's not nil
}

func TestConnectGetLibrary_WithBooks(t *testing.T) {
	book := addTestBook(t, "Test Book")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.GetLibraryRequest{})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetLibrary(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg.Library)
	assert.NotEmpty(t, resp.Msg.Library.Wishlist, "book should be in wishlist")
}

func TestConnectGetBooksProgress_DefaultRange(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&backlogv1.GetBooksProgressRequest{},
	)
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetBooksProgress(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg.Progress)
	assert.NotEmpty(t, resp.Msg.Progress.DateStart)
	assert.NotEmpty(t, resp.Msg.Progress.DateEnd)
}

func TestConnectGetBooksProgress_WithDateRange(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.GetBooksProgressRequest{
		DateStart: "2024-01-01",
		DateEnd:   "2024-12-31",
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetBooksProgress(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg.Progress)
	assert.Equal(t, "2024-01-01", resp.Msg.Progress.DateStart)
	assert.Equal(t, "2024-12-31", resp.Msg.Progress.DateEnd)
}

func TestConnectGetSteam(t *testing.T) {
	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&backlogv1.GetSteamRequest{},
	)
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetSteam(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	if resp.Msg.Steam != nil {
		assert.GreaterOrEqual(t, int(resp.Msg.Steam.TotalBacklog), 0)
		assert.NotNil(t, resp.Msg.Steam.Distribution)
	}
}

func TestConnectGetSteamGame(t *testing.T) {
	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.GetSteamGameRequest{
		GameId: 570,
	})
	req.Header().Set("Cookie", accessToken.String())

	// The mock may not have every game, so we just check the request can be called
	// without panicking. It may return an error if the game doesn't exist in the mock.
	_, _ = client.GetSteamGame(ctx, req)
}

func TestConnectGetSteamGame_WithSeededData(t *testing.T) {
	seedSteamData(t)

	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.GetSteamGameRequest{
		GameId: 1,
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetSteamGame(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg.Data)
	assert.NotNil(t, resp.Msg.Data.Game)
}

func TestConnectGetRecentlyActiveGames(t *testing.T) {
	seedSteamData(t)

	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.GetRecentlyActiveGamesRequest{})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetRecentlyActiveGames(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.Msg.Games)

	found := false
	for _, g := range resp.Msg.Games {
		if g.Id == 1 {
			found = true
			assert.GreaterOrEqual(t, g.RecentUnlocks, int32(1))
			assert.NotEmpty(t, g.LastUnlockedAt)
		}
	}
	assert.True(t, found, "seeded game should appear in recent activity")
}

func TestConnectGetSteamDistribution_ValidBucket(t *testing.T) {
	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.GetSteamDistributionRequest{
		Bucket: 0,
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetSteamDistribution(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	if resp.Msg.Data != nil {
		assert.Equal(t, "0–9%", resp.Msg.Data.Label)
	}
}

func TestConnectGetSteamDistribution_InvalidBucket(t *testing.T) {
	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.GetSteamDistributionRequest{
		Bucket: 99,
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.GetSteamDistribution(ctx, req)
	assert.Error(t, err)
	var connectErr *connect.Error
	assert.True(t, errors.As(err, &connectErr))
}

// TestConnectRefreshSteamGame_GameNotFound verifies that RefreshSteamGame
// returns an error when the requested game does not exist in the database (the
// no-credentials no-op still falls through to GetGameByID which fails).
func TestConnectRefreshSteamGame_GameNotFound(t *testing.T) {
	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Game 999999 was never seeded; testApp has empty Steam creds so SyncGame
	// is a no-op, and GetGameByID returns an error.
	req := connect.NewRequest(&backlogv1.RefreshSteamGameRequest{GameId: 999999})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.RefreshSteamGame(ctx, req)
	assert.Error(t, err)
	var connectErr *connect.Error
	assert.True(t, errors.As(err, &connectErr))
}

func TestConnectRefreshSteamGame_WithSeededData(t *testing.T) {
	seedSteamData(t)

	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.RefreshSteamGameRequest{GameId: 1})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.RefreshSteamGame(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Msg.Data)
	assert.NotNil(t, resp.Msg.Data.Game)
	assert.NotEmpty(t, resp.Msg.Data.Achievements)
}

func TestConnectSearchLibrary_Empty(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.SearchLibraryRequest{
		Query: "",
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.SearchLibrary(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Msg.Books)
}

func TestConnectSearchLibrary_WithResults(t *testing.T) {
	book := addTestBook(t, "SearchableBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.SearchLibraryRequest{
		Query: "SearchableBook",
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.SearchLibrary(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.Msg.Books)
}

func TestConnectSearchExternal_Empty(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.SearchExternalRequest{
		Query: "",
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.SearchExternal(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Msg.Results)
}

func TestConnectSearchExternal_WithQuery(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.SearchExternalRequest{
		Query: "Harry Potter",
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.SearchExternal(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	// Results may be empty depending on mock data
}

func TestConnectAddBook(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&backlogv1.AddBookRequest{
			Provider:    "manual",
			ProviderId:  "test-add-book",
			Title:       "New Book to Add",
			Author:      "Test Author",
			Status:      models.StatusToRead,
			Isbn13:      "9780140449112",
			CoverUrl:    "https://example.com/cover.jpg",
			OwnPhysical: true,
		},
	)
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.AddBook(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg)
}

func TestConnectUpdateBookStatus(t *testing.T) {
	book := addTestBook(t, "StatusChangeBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&backlogv1.UpdateBookStatusRequest{
			BookId:    book.BookID.String(),
			Status:    models.StatusReading,
			Favourite: false,
			Rating:    "4",
		},
	)
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.UpdateBookStatus(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg)
}

func TestConnectUpdateBookStatus_MarkRead(t *testing.T) {
	book := addTestBook(t, "ReadBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&backlogv1.UpdateBookStatusRequest{
			BookId:    book.BookID.String(),
			Status:    models.StatusRead,
			Favourite: false,
			Rating:    "5",
		},
	)
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.UpdateBookStatus(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg)
}

func TestConnectUpdateProgress_Pages(t *testing.T) {
	book := addTestBook(t, "ProgressPagesBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&backlogv1.UpdateProgressRequest{
			BookId:       book.BookID.String(),
			ProgressMode: models.ProgressModePages,
			CurrentPage:  120,
		},
	)
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.UpdateProgress(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg)

	saved, err := testApp.Repositories.Books.GetUserBook(ctx, userID, book.BookID)
	require.NoError(t, err)
	assert.Equal(t, models.ProgressModePages, saved.ProgressMode)
	assert.Equal(t, 120, saved.CurrentPage)
}

func TestConnectUpdateProgress_PercentClamped(t *testing.T) {
	book := addTestBook(t, "ProgressPercentBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&backlogv1.UpdateProgressRequest{
			BookId:          book.BookID.String(),
			ProgressMode:    models.ProgressModePercent,
			ProgressPercent: 150,
		},
	)
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.UpdateProgress(ctx, req)
	require.NoError(t, err)

	saved, err := testApp.Repositories.Books.GetUserBook(ctx, userID, book.BookID)
	require.NoError(t, err)
	assert.Equal(t, models.ProgressModePercent, saved.ProgressMode)
	assert.Equal(t, 100, saved.ProgressPercent)
}

func TestConnectUpdateProgress_NegativeValuesClampedToZero(t *testing.T) {
	book := addTestBook(t, "ProgressNegativeBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&backlogv1.UpdateProgressRequest{
			BookId:          book.BookID.String(),
			ProgressMode:    models.ProgressModePercent,
			CurrentPage:     -5,
			ProgressPercent: -10,
		},
	)
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.UpdateProgress(ctx, req)
	require.NoError(t, err)

	saved, err := testApp.Repositories.Books.GetUserBook(ctx, userID, book.BookID)
	require.NoError(t, err)
	assert.Equal(t, 0, saved.CurrentPage)
	assert.Equal(t, 0, saved.ProgressPercent)
}

func TestConnectUpdateProgress_InvalidBookID(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&backlogv1.UpdateProgressRequest{
			BookId:       "not-a-uuid",
			ProgressMode: models.ProgressModePages,
		},
	)
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.UpdateProgress(ctx, req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestConnectUpdateProgress_InvalidMode(t *testing.T) {
	book := addTestBook(t, "ProgressInvalidModeBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&backlogv1.UpdateProgressRequest{
			BookId:       book.BookID.String(),
			ProgressMode: "chapters",
		},
	)
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.UpdateProgress(ctx, req)
	require.Error(t, err)
}

func TestConnectToggleTag_AddTag(t *testing.T) {
	book := addTestBook(t, "TagBook1")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.ToggleTagRequest{
		BookId: book.BookID.String(),
		Tag:    "fantasy",
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.ToggleTag(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg)
}

func TestConnectToggleTag_RemoveTag(t *testing.T) {
	book := addTestBook(t, "TagBook2")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// First add a tag
	addReq := connect.NewRequest(&backlogv1.ToggleTagRequest{
		BookId: book.BookID.String(),
		Tag:    "mystery",
	})
	addReq.Header().Set("Cookie", accessToken.String())
	_, err := client.ToggleTag(ctx, addReq)
	require.NoError(t, err)

	// Then remove it
	removeReq := connect.NewRequest(&backlogv1.ToggleTagRequest{
		BookId: book.BookID.String(),
		Tag:    "mystery",
	})
	removeReq.Header().Set("Cookie", accessToken.String())
	resp, err := client.ToggleTag(ctx, removeReq)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg)
}

func TestConnectToggleTag_EmptyTag(t *testing.T) {
	book := addTestBook(t, "TagBook3")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.ToggleTagRequest{
		BookId: book.BookID.String(),
		Tag:    "",
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.ToggleTag(ctx, req)
	assert.Error(t, err)
	var connectErr *connect.Error
	assert.True(t, errors.As(err, &connectErr))
}

func TestConnectImportBooks(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.ImportBooksRequest{
		CsvData: []byte(goodreadsCSVForImport),
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.ImportBooks(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg)
	assert.GreaterOrEqual(t, resp.Msg.ImportedCount, int32(0))
}
