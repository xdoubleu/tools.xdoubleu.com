package backlog_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/backlog"
	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/objectstore"
	"tools.xdoubleu.com/apps/backlog/pkg/openlibrary"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
	backlogv1 "tools.xdoubleu.com/gen/backlog/v1"
	backlogv1connect "tools.xdoubleu.com/gen/backlog/v1/backlogv1connect"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

// TestNew_ReturnsApp covers the production New constructor that wires real
// steam/Open Library factories.
func TestNew_ReturnsApp(t *testing.T) {
	bl := backlog.New(
		context.Background(),
		sharedmocks.NewMockedAuthService(userID),
		testApp.Logger,
		testCfg,
		testDB,
	)
	require.NotNil(t, bl)
}

// TestStart_RegistersJobs covers Start → setJobs.
func TestStart_RegistersJobs(t *testing.T) {
	bl := backlog.NewInner(
		context.Background(),
		sharedmocks.NewMockedAuthService(userID),
		testApp.Logger,
		testCfg,
		testDB,
		backlog.Clients{
			SteamFactory: func(_ string) steam.Client {
				return nil
			},
			OpenLibrary:      nil,
			ObjectStore:      objectstore.NewFake(),
			KoboStoreBaseURL: "",
			PublicAPIBaseURL: "",
		},
	)
	require.NotNil(t, bl)
	err := bl.Start()
	require.NoError(t, err)
}

// addTestBookWithISBN adds a book with a unique ISBN so it gets its own DB row.
func addTestBookWithISBN(t *testing.T, title, isbn string) *models.UserBook {
	t.Helper()
	cover := "https://example.com/cover.jpg"
	ext := openlibrary.ExternalBook{ //nolint:exhaustruct //optional fields not needed
		Provider:   "manual",
		ProviderID: fmt.Sprintf("cov-%s-%s", title, uuid.New()),
		Title:      title,
		Authors:    []string{"Coverage Author"},
		ISBN13:     &isbn,
		CoverURL:   &cover,
	}
	ub, err := testApp.Services.Books.AddToLibrary(
		context.Background(), userID, ext, models.StatusToRead, []string{},
	)
	require.NoError(t, err)
	require.NotNil(t, ub)
	return ub
}

// TestConnectGetLibrary_WithVariousBooksAndShelves covers:
//   - StatusReading and StatusRead cases in buildLibraryData
//   - int32PtrFromInt16 nil path (fresh book, no rating set)
//   - int32PtrFromInt16 non-nil path (book with rating "4")
//   - groupByTags inner loop and protoBookshelves loop body
//   - slices.SortFunc comparison body (return -1 and return 1) via 3+ shelves
func TestConnectGetLibrary_WithVariousBooksAndShelves(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	uid := uuid.New().String()[:8]

	// Three books with distinct ISBNs so each gets a separate DB row.
	bookA := addTestBookWithISBN(t, "CovReading-"+uid, "9780000000001")
	bookB := addTestBookWithISBN(t, "CovRead-"+uid, "9780000000002")
	bookC := addTestBookWithISBN(t, "CovNilRating-"+uid, "9780000000003")

	// Mark bookA as currently-reading (covers StatusReading in buildLibraryData)
	readingReq := connect.NewRequest(&backlogv1.UpdateBookStatusRequest{
		BookId: bookA.BookID.String(), Status: models.StatusReading,
	})
	readingReq.Header().Set("Cookie", accessToken.String())
	_, err := newBooksTestClient(t).UpdateBookStatus(ctx, readingReq)
	require.NoError(t, err)

	// Mark bookB as read with rating (covers StatusRead + int32PtrFromInt16 non-nil)
	readReq := connect.NewRequest(&backlogv1.UpdateBookStatusRequest{
		BookId: bookB.BookID.String(), Status: models.StatusRead, Rating: "4",
	})
	readReq.Header().Set("Cookie", accessToken.String())
	_, err = newBooksTestClient(t).UpdateBookStatus(ctx, readReq)
	require.NoError(t, err)

	// bookC stays as to-read with nil rating, covering int32PtrFromInt16 nil branch.

	// Add 3 distinct tags (one per book) to create 3 shelves, triggering
	// the SortFunc comparison for both return -1 and return 1 paths.
	for i, tag := range []string{"aaa-shelf", "mmm-shelf", "zzz-shelf"} {
		book := []*models.UserBook{bookA, bookB, bookC}[i]
		tagReq := connect.NewRequest(&backlogv1.ToggleTagRequest{
			BookId: book.BookID.String(), Tag: tag,
		})
		tagReq.Header().Set("Cookie", accessToken.String())
		_, err = newBooksTestClient(t).ToggleTag(ctx, tagReq)
		require.NoError(t, err)
	}

	// GetLibrary exercises all the paths above.
	libReq := connect.NewRequest(&backlogv1.GetLibraryRequest{})
	libReq.Header().Set("Cookie", accessToken.String())
	resp, err := newBooksTestClient(t).GetLibrary(ctx, libReq)
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg.Library)
	assert.NotEmpty(t, resp.Msg.Library.Finished)
	assert.NotEmpty(t, resp.Msg.Library.Shelves)
}

// TestConnectUpdateBookStatus_ZeroRating covers parseRating's "0" early-return branch.
func TestConnectUpdateBookStatus_ZeroRating(t *testing.T) {
	book := addTestBook(t, "ZeroRatingBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.UpdateBookStatusRequest{
		BookId:    book.BookID.String(),
		Status:    models.StatusReading,
		Favourite: false,
		Rating:    "0",
	})
	req.Header().Set("Cookie", accessToken.String())
	_, err := client.UpdateBookStatus(ctx, req)
	require.NoError(t, err)
}

// TestConnectUpdateBookStatus_NegativeRating covers parseRating's error/n<=0 branch.
func TestConnectUpdateBookStatus_NegativeRating(t *testing.T) {
	book := addTestBook(t, "NegativeRatingBook")
	require.NotNil(t, book)

	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.UpdateBookStatusRequest{
		BookId:    book.BookID.String(),
		Status:    models.StatusReading,
		Favourite: false,
		Rating:    "-1",
	})
	req.Header().Set("Cookie", accessToken.String())
	_, err := client.UpdateBookStatus(ctx, req)
	require.NoError(t, err)
}

// twoAchievementsMock returns a game with two player achievements but only one
// has a global percentage entry, causing nil GlobalPercent on the second — which
// exercises the nil-branches in GetSteamGame's sort.Slice comparison function.
type twoAchievementsMock struct{}

func (twoAchievementsMock) GetOwnedGames(
	_ context.Context,
	_ string,
) (*steam.OwnedGamesResponse, error) {
	return &steam.OwnedGamesResponse{
		Response: steam.OwnedGamesResponseData{
			GameCount: 1,
			Games: []steam.Game{
				{ //nolint:exhaustruct //only required fields
					AppID:                    7,
					Name:                     "two-ach game",
					HasCommunityVisibleStats: true,
				},
			},
		},
	}, nil
}

func (twoAchievementsMock) GetPlayerAchievements(
	_ context.Context,
	steamID string,
	_ int,
) (*steam.AchievementsResponse, error) {
	return &steam.AchievementsResponse{
		PlayerStats: steam.PlayerStats{
			Success:  true,
			SteamID:  steamID,
			GameName: "two-ach game",
			Achievements: []steam.Achievement{
				{
					APIName:     "ACH_A",
					Achieved:    1,
					UnlockTime:  0,
					Name:        "Alpha",
					Description: "",
				},
				{
					APIName:     "ACH_B",
					Achieved:    0,
					UnlockTime:  0,
					Name:        "Beta",
					Description: "",
				},
			},
		},
	}, nil
}

func (twoAchievementsMock) GetSchemaForGame(
	_ context.Context,
	_ int,
) (*steam.GetSchemaForGameResponse, error) {
	//nolint:exhaustruct //skip
	return &steam.GetSchemaForGameResponse{}, nil
}

func (twoAchievementsMock) GetGlobalAchievementPercentagesForApp(
	_ context.Context,
	_ int,
) (*steam.GlobalAchievementPercentagesResponse, error) {
	// Only ACH_A has a global percent; ACH_B will get nil GlobalPercent.
	//nolint:exhaustruct //anonymous inner struct initialised via field assignment
	resp := steam.GlobalAchievementPercentagesResponse{}
	resp.AchievementPercentages.Achievements = []steam.GlobalAchievementPercent{
		{Name: "ACH_A", Percent: "75.0"},
	}
	return &resp, nil
}

// TestConnectGetSteamGame_SortBranches seeds a game with two achievements (one
// with GlobalPercent, one without) and calls GetSteamGame, covering the
// nil-GlobalPercent branches in the sort.Slice comparison.
func TestConnectGetSteamGame_SortBranches(t *testing.T) {
	const isolatedUser = "sort-branch-test-user"

	app2 := backlog.NewInner(
		context.Background(),
		sharedmocks.NewMockedAuthService(isolatedUser),
		testApp.Logger,
		testCfg,
		testDB,
		backlog.Clients{
			SteamFactory: func(_ string) steam.Client {
				return twoAchievementsMock{}
			},
			OpenLibrary:      nil,
			ObjectStore:      objectstore.NewFake(),
			KoboStoreBaseURL: "",
			PublicAPIBaseURL: "",
		},
	)

	err := app2.SaveIntegrations(
		context.Background(),
		isolatedUser,
		backlog.Integrations{
			SteamUserID: "76561197960287930",
		},
	)
	require.NoError(t, err)

	err = app2.Services.Steam.SyncUser(context.Background(), isolatedUser)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.steam_games WHERE user_id = $1`, isolatedUser)
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.integrations WHERE user_id = $1`, isolatedUser)
	})

	ts := httptest.NewServer(testhelper.BuildMux(app2))
	t.Cleanup(ts.Close)
	client := backlogv1connect.NewGamesServiceClient(
		http.DefaultClient,
		ts.URL,
		connect.WithHTTPGet(),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.GetSteamGameRequest{GameId: 7})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetSteamGame(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg.Data)
	assert.Len(t, resp.Msg.Data.Achievements, 2)
}

// TestConnectRefreshSteamGame_SortBranches seeds a game with two achievements
// (one with GlobalPercent, one without) and calls RefreshSteamGame, covering
// the nil-GlobalPercent branches in the sort.Slice comparison and the full
// happy path of the handler.
func TestConnectRefreshSteamGame_SortBranches(t *testing.T) {
	const isolatedUser = "refresh-sort-branch-user"

	app2 := backlog.NewInner(
		context.Background(),
		sharedmocks.NewMockedAuthService(isolatedUser),
		testApp.Logger,
		testCfg,
		testDB,
		backlog.Clients{
			SteamFactory:     func(_ string) steam.Client { return twoAchievementsMock{} },
			OpenLibrary:      nil,
			ObjectStore:      objectstore.NewFake(),
			KoboStoreBaseURL: "",
			PublicAPIBaseURL: "",
		},
	)
	require.NoError(t, app2.SaveIntegrations(
		context.Background(),
		isolatedUser,
		backlog.Integrations{
			SteamUserID: "76561197960287930",
		},
	))
	require.NoError(t, app2.Services.Steam.SyncUser(context.Background(), isolatedUser))

	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.steam_achievements WHERE user_id = $1`, isolatedUser)
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.steam_games WHERE user_id = $1`, isolatedUser)
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.user_integrations WHERE user_id = $1`, isolatedUser)
	})

	ts := httptest.NewServer(testhelper.BuildMux(app2))
	t.Cleanup(ts.Close)
	client := backlogv1connect.NewGamesServiceClient(
		http.DefaultClient,
		ts.URL,
		connect.WithHTTPGet(),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.RefreshSteamGameRequest{GameId: 7})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.RefreshSteamGame(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg.Data)
	assert.Len(t, resp.Msg.Data.Achievements, 2)
}

// fourAchievementsMock is a Steam client that returns four achievements for game
// 9: two with GlobalPercent and two without. This exercises every branch of the
// sort.Slice comparator inside RefreshSteamGame (and GetSteamGame):
//
//	*pi > *pj       — both achievements have a percent
//	pi == nil       — achievement at index i has no percent
//	pj == nil       — achievement at index j has no percent (pi != nil)
//	both nil        — both achievements at i and j have no percent (DisplayName compare)
type fourAchievementsMock struct{}

func (fourAchievementsMock) GetOwnedGames(
	_ context.Context,
	_ string,
) (*steam.OwnedGamesResponse, error) {
	return &steam.OwnedGamesResponse{
		Response: steam.OwnedGamesResponseData{
			GameCount: 1,
			Games: []steam.Game{
				{ //nolint:exhaustruct //only required fields
					AppID:                    9,
					Name:                     "four-ach game",
					HasCommunityVisibleStats: true,
				},
			},
		},
	}, nil
}

func (fourAchievementsMock) GetPlayerAchievements(
	_ context.Context,
	steamID string,
	_ int,
) (*steam.AchievementsResponse, error) {
	return &steam.AchievementsResponse{
		PlayerStats: steam.PlayerStats{
			Success:  true,
			SteamID:  steamID,
			GameName: "four-ach game",
			Achievements: []steam.Achievement{
				// Two with global percents (different values → exercises *pi > *pj)
				{
					APIName:     "ACH_HIGH",
					Achieved:    0,
					UnlockTime:  0,
					Name:        "",
					Description: "",
				},
				{
					APIName:     "ACH_LOW",
					Achieved:    0,
					UnlockTime:  0,
					Name:        "",
					Description: "",
				},
				// Two without global percents (exercises both-nil DisplayName branch)
				{
					APIName:     "ACH_NIL_A",
					Achieved:    0,
					UnlockTime:  0,
					Name:        "",
					Description: "",
				},
				{
					APIName:     "ACH_NIL_B",
					Achieved:    0,
					UnlockTime:  0,
					Name:        "",
					Description: "",
				},
			},
		},
	}, nil
}

func (fourAchievementsMock) GetSchemaForGame(
	_ context.Context,
	_ int,
) (*steam.GetSchemaForGameResponse, error) {
	//nolint:exhaustruct //empty schema; DisplayNames default to ""
	return &steam.GetSchemaForGameResponse{}, nil
}

func (fourAchievementsMock) GetGlobalAchievementPercentagesForApp(
	_ context.Context,
	_ int,
) (*steam.GlobalAchievementPercentagesResponse, error) {
	//nolint:exhaustruct //anonymous inner struct initialised via field assignment
	resp := steam.GlobalAchievementPercentagesResponse{}
	resp.AchievementPercentages.Achievements = []steam.GlobalAchievementPercent{
		{Name: "ACH_HIGH", Percent: "90.0"},
		{Name: "ACH_LOW", Percent: "10.0"},
		// ACH_NIL_A and ACH_NIL_B intentionally omitted → nil GlobalPercent
	}
	return &resp, nil
}

// TestConnectRefreshSteamGame_AllSortBranches seeds a game with four achievements
// (two with GlobalPercent, two without) and calls RefreshSteamGame, covering all
// remaining sort.Slice comparator branches: *pi > *pj, pj == nil, and both-nil
// DisplayName comparison.
func TestConnectRefreshSteamGame_AllSortBranches(t *testing.T) {
	const isolatedUser = "refresh-all-sort-user"
	ctx := context.Background()

	app2 := backlog.NewInner(
		ctx,
		sharedmocks.NewMockedAuthService(isolatedUser),
		testApp.Logger,
		testCfg,
		testDB,
		backlog.Clients{
			SteamFactory:     func(_ string) steam.Client { return fourAchievementsMock{} },
			OpenLibrary:      nil,
			ObjectStore:      objectstore.NewFake(),
			KoboStoreBaseURL: "",
			PublicAPIBaseURL: "",
		},
	)
	require.NoError(t, app2.SaveIntegrations(
		ctx,
		isolatedUser,
		backlog.Integrations{
			SteamUserID: "76561197960287930",
		},
	))
	require.NoError(t, app2.Services.Steam.SyncUser(ctx, isolatedUser))

	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.steam_achievements WHERE user_id = $1`, isolatedUser)
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.steam_games WHERE user_id = $1`, isolatedUser)
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.user_integrations WHERE user_id = $1`, isolatedUser)
	})

	ts := httptest.NewServer(testhelper.BuildMux(app2))
	t.Cleanup(ts.Close)
	client := backlogv1connect.NewGamesServiceClient(
		http.DefaultClient,
		ts.URL,
		connect.WithHTTPGet(),
	)
	reqCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	req := connect.NewRequest(&backlogv1.RefreshSteamGameRequest{GameId: 9})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.RefreshSteamGame(reqCtx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg.Data)
	assert.Len(t, resp.Msg.Data.Achievements, 4,
		"all four achievements should be returned after refresh")
}

// TestConnectRefreshSteamGame_SyncError verifies that RefreshSteamGame returns
// CodeInternal when SyncGame fails (e.g. Steam schema fetch error).
func TestConnectRefreshSteamGame_SyncError(t *testing.T) {
	const isolatedUser = "refresh-sync-error-user"
	const gameID = 7

	app2 := backlog.NewInner(
		context.Background(),
		sharedmocks.NewMockedAuthService(isolatedUser),
		testApp.Logger,
		testCfg,
		testDB,
		backlog.Clients{
			SteamFactory: func(_ string) steam.Client {
				// Uses syncFakeClient with schemaErr so fetchAchievementsForGame
				// fails, causing SyncGame to return an error.
				return syncFakeClient{
					games:     []steam.Game{},
					playerAch: map[int][]steam.Achievement{},
					schemaErr: map[int]bool{gameID: true},
				}
			},
			OpenLibrary:      nil,
			ObjectStore:      objectstore.NewFake(),
			KoboStoreBaseURL: "",
			PublicAPIBaseURL: "",
		},
	)
	require.NoError(t, app2.SaveIntegrations(
		context.Background(),
		isolatedUser,
		backlog.Integrations{
			SteamUserID: "76561197960287930",
		},
	))

	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM backlog.user_integrations WHERE user_id = $1`, isolatedUser)
	})

	ts := httptest.NewServer(testhelper.BuildMux(app2))
	t.Cleanup(ts.Close)
	client := backlogv1connect.NewGamesServiceClient(
		http.DefaultClient,
		ts.URL,
		connect.WithHTTPGet(),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&backlogv1.RefreshSteamGameRequest{GameId: int32(gameID)},
	)
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.RefreshSteamGame(ctx, req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.True(t, errors.As(err, &connectErr))
	assert.Equal(t, connect.CodeInternal, connectErr.Code())
}

// TestConnectGetLibrary_FormatsPopulated asserts that a book with an uploaded
// PDF file has its Formats field populated on the GetLibrary response.
func TestConnectGetLibrary_FormatsPopulated(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	uid := uuid.New().String()[:8]
	book := addTestBookWithISBN(t, "FormatsBook-"+uid, "9780000099001")

	// Insert a ready PDF book_file row directly via the repository so we don't
	// need a real object store upload.
	pdfFile := models.BookFile{ //nolint:exhaustruct //optional nullable fields omitted
		BookID:     book.BookID,
		UserID:     userID,
		Format:     models.FileFormatPDF,
		StorageKey: "users/test/books/pdf/formats-lib.pdf",
		SizeBytes:  512,
		Status:     models.FileStatusReady,
	}
	_, err := testApp.Repositories.BookFiles.Insert(ctx, pdfFile)
	require.NoError(t, err)

	libReq := connect.NewRequest(&backlogv1.GetLibraryRequest{})
	libReq.Header().Set("Cookie", accessToken.String())
	resp, err := newBooksTestClient(t).GetLibrary(ctx, libReq)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Library)

	// Find our book in the wishlist (default status is to-read).
	var found bool
	for _, ub := range resp.Msg.Library.Wishlist {
		if ub.BookId == book.BookID.String() {
			assert.Contains(t, ub.Formats, models.FileFormatPDF)
			assert.NotContains(t, ub.Formats, models.FileFormatEPUB)
			found = true
			break
		}
	}
	assert.True(t, found, "expected book in wishlist")
}

// TestConnectResyncOpenLibrary_Success verifies that an authenticated user can
// trigger the resync endpoint and get a 200 response.
func TestConnectResyncOpenLibrary_Success(t *testing.T) {
	client := newBooksTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&backlogv1.ResyncOpenLibraryRequest{})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.ResyncOpenLibrary(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// TestResyncAllFromOpenLibrary_Service exercises the service layer end-to-end
// against the real DB. It seeds a book that already has a cover_url and
// verifies that resync does not touch its R2 cover cache — resync is
// additive-only and must never clobber data that already exists.
//
// The cache-bust path (no existing cover → OL provides one → cache busted) is
// covered exhaustively by the unit tests in
// internal/services/book_resync_test.go, which avoid the AddToLibrary
// enrichment that always fills in a cover via the mock OL client.
func TestResyncAllFromOpenLibrary_Service(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	uid := uuid.New().String()[:8]
	book := addTestBookWithISBN(t, "ResyncTest-"+uid, "9780000099001")

	// Pre-seed a cached cover and a missing marker so we can verify neither is
	// deleted when the book already has a cover_url.
	coverKey := "books/" + book.BookID.String() + "/cover.jpg"
	missingKey := "books/" + book.BookID.String() + "/cover.missing"
	require.NoError(
		t,
		fakeStore.Put(ctx, coverKey, bytes.NewReader([]byte("img")), 3, "image/jpeg"),
	)
	require.NoError(
		t,
		fakeStore.Put(
			ctx,
			missingKey,
			bytes.NewReader([]byte{}),
			0,
			"application/octet-stream",
		),
	)

	exists, err := fakeStore.Exists(ctx, coverKey)
	require.NoError(t, err)
	require.True(t, exists, "cover should be in store before resync")

	n, resyncErr := testApp.Services.Books.ResyncAllFromOpenLibrary(
		ctx,
		testApp.Logger,
		nil,
	)
	require.NoError(t, resyncErr)
	assert.GreaterOrEqual(t, n, 0, "resync should complete without error")

	// Cover already existed — cache must NOT be disturbed.
	exists, err = fakeStore.Exists(ctx, coverKey)
	require.NoError(t, err)
	assert.True(
		t, exists,
		"cover.jpg cache must be preserved when the book already has a cover URL",
	)

	missing, err := fakeStore.Exists(ctx, missingKey)
	require.NoError(t, err)
	assert.True(
		t, missing,
		"cover.missing marker must be preserved when the book already has a cover URL",
	)
}
