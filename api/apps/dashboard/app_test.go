package dashboard_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/books"
	booksobjectstore "tools.xdoubleu.com/apps/books/pkg/objectstore"
	bookswebfetch "tools.xdoubleu.com/apps/books/pkg/webfetch"
	"tools.xdoubleu.com/apps/dashboard"
	"tools.xdoubleu.com/apps/feeds"
	feedswebfetch "tools.xdoubleu.com/apps/feeds/pkg/webfetch"
	"tools.xdoubleu.com/apps/games"
	"tools.xdoubleu.com/apps/games/pkg/steam"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mailer"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/notifications"
	sharedrepos "tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/testhelper"
)

//nolint:gochecknoglobals //needed for tests
var (
	testApp   *dashboard.Dashboard
	testGames *games.Games
	testBooks *books.Books
	testFeeds *feeds.Feeds
	testDB    postgres.DB
	testCfg   config.Config
	userID    = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"
)

const mockGameName = "test-game"

// fakeSteamClient is a small local stand-in for steam.Client — the real mock
// lives in apps/games/internal/mocks, which this package (outside the games
// app's own tree) cannot import (Go internal-package visibility).
type fakeSteamClient struct{}

func (fakeSteamClient) GetOwnedGames(
	_ context.Context,
	_ string,
) (*steam.OwnedGamesResponse, error) {
	return &steam.OwnedGamesResponse{
		Response: steam.OwnedGamesResponseData{
			GameCount: 1,
			Games: []steam.Game{
				{
					AppID:                    1,
					Name:                     mockGameName,
					ImgIconURL:               "",
					ImgLogoURL:               "",
					HasCommunityVisibleStats: true,
					PlaytimeForever:          0,
					RtimeLastPlayed:          time.Now().UTC().Unix(),
				},
			},
		},
	}, nil
}

func (fakeSteamClient) GetPlayerAchievements(
	_ context.Context,
	steamID string,
	_ int,
) (*steam.AchievementsResponse, error) {
	return &steam.AchievementsResponse{
		PlayerStats: steam.PlayerStats{
			Success:  true,
			SteamID:  steamID,
			GameName: mockGameName,
			Achievements: []steam.Achievement{
				{
					APIName:     "TEST",
					Achieved:    1,
					UnlockTime:  time.Now().UTC().Unix(),
					Name:        mockGameName,
					Description: "Hello, World!",
				},
			},
		},
	}, nil
}

func (fakeSteamClient) GetSchemaForGame(
	_ context.Context,
	_ int,
) (*steam.GetSchemaForGameResponse, error) {
	//nolint:exhaustruct //skip
	return &steam.GetSchemaForGameResponse{}, nil
}

func (fakeSteamClient) GetGlobalAchievementPercentagesForApp(
	_ context.Context,
	_ int,
) (*steam.GlobalAchievementPercentagesResponse, error) {
	//nolint:exhaustruct //anonymous inner struct initialised via field assignment
	resp := steam.GlobalAchievementPercentagesResponse{}
	resp.AchievementPercentages.Achievements = []steam.GlobalAchievementPercent{
		{Name: "TEST", Percent: "42.0"},
	}
	return &resp, nil
}

// fakeBooksWebFetchClient/fakeFeedsWebFetchClient are small local stand-ins
// for their apps' respective webfetch.Client. Dashboard's own tests never
// trigger a real fetch — feeds items are seeded directly via SQL — so every
// call errors.
type fakeBooksWebFetchClient struct{}

func (fakeBooksWebFetchClient) Get(
	_ context.Context,
	_ string,
	_ bookswebfetch.Options,
) (*bookswebfetch.Result, error) {
	return nil, bookswebfetch.ErrNetwork
}

type fakeFeedsWebFetchClient struct{}

func (fakeFeedsWebFetchClient) Get(
	_ context.Context,
	_ string,
	_ feedswebfetch.Options,
) (*feedswebfetch.Result, error) {
	return nil, feedswebfetch.ErrNetwork
}

func TestMain(m *testing.M) {
	testCfg = testhelper.NewTestConfig()
	testCfg.SteamAPIKey = "test-steam-api-key"
	testCfg.EmailInboundDomain = "mail.example.com"

	postgresDB := testhelper.ConnectTestDB(testCfg.DBDsn)
	testDB = postgresDB

	auth := sharedmocks.NewMockedAuthService(userID)
	logger := logging.NewNopLogger()

	testBooks = books.NewInner(auth, logger, testCfg, postgresDB, books.Clients{
		UniCat:           nil,
		Hardcover:        nil,
		ObjectStore:      booksobjectstore.NewFake(),
		WebFetch:         fakeBooksWebFetchClient{},
		KoboStoreBaseURL: "",
		PublicAPIBaseURL: "",
	})
	if err := testBooks.ApplyMigrations(context.Background(), postgresDB); err != nil {
		panic(err)
	}

	testFeeds = feeds.NewInner(
		auth,
		logger,
		testCfg,
		postgresDB,
		fakeFeedsWebFetchClient{},
		notifications.New(context.Background(), logger, mailer.New("", "", "")),
		sharedrepos.NewAppUsersRepository(postgresDB),
	)
	if err := testFeeds.ApplyMigrations(context.Background(), postgresDB); err != nil {
		panic(err)
	}

	testGames = games.NewInner(
		auth,
		logger,
		testCfg,
		postgresDB,
		func(_ string) steam.Client { return fakeSteamClient{} },
	)
	if err := testGames.ApplyMigrations(context.Background(), postgresDB); err != nil {
		panic(err)
	}

	testApp = dashboard.New(
		auth,
		logger,
		testCfg,
		postgresDB,
		testGames,
		testBooks,
		testFeeds,
	)
	if err := testApp.ApplyMigrations(context.Background(), postgresDB); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

func getRoutes() http.Handler {
	return testhelper.BuildMux(testApp)
}

func TestGetDisplayName(t *testing.T) {
	assert.Equal(t, "Dashboards", testApp.GetDisplayName())
}
