package backlog_test

import (
	"context"
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
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
	backlogv1 "tools.xdoubleu.com/gen/backlog/v1"
	backlogv1connect "tools.xdoubleu.com/gen/backlog/v1/backlogv1connect"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

// TestNew_ReturnsApp covers the production New constructor that wires real
// steam/hardcover factories.
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
			HardcoverFactory: func(_ string) hardcover.Client {
				return nil
			},
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
	ext := hardcover.ExternalBook{ //nolint:exhaustruct //optional fields not needed
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
			HardcoverFactory: func(_ string) hardcover.Client {
				return nil
			},
		},
	)

	err := app2.SaveIntegrations(
		context.Background(),
		isolatedUser,
		backlog.Integrations{ //nolint:exhaustruct //only steam needed
			SteamAPIKey: "test-key",
			SteamUserID: "76561197960287930",
		},
	)
	require.NoError(t, err)

	_, err = app2.Services.Steam.ImportOwnedGames(context.Background(), isolatedUser)
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
