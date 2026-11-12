package dashboard_test

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/books"
	booksobjectstore "tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/apps/dashboard"
	"tools.xdoubleu.com/apps/factory"
	"tools.xdoubleu.com/apps/feeds"
	"tools.xdoubleu.com/apps/games"
	"tools.xdoubleu.com/apps/games/pkg/steam"
	"tools.xdoubleu.com/apps/learningpaths"
	"tools.xdoubleu.com/apps/mealplans"
	"tools.xdoubleu.com/apps/movies"
	"tools.xdoubleu.com/apps/recipes"
	"tools.xdoubleu.com/apps/shoppinglist"
	sharedauth "tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/crypto"
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
	testToday dashboard.TodayApps
	testGames *games.Games
	testBooks *books.Books
	testFeeds *feeds.Feeds
	testDB    postgres.DB
	testCfg   config.Config
	userID    = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"
)

const mockGameName = "test-game"

// fakeSteamClient stands in for steam.Client; the games mock is internal.
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

func TestMain(m *testing.M) {
	testCfg = testhelper.NewTestConfig()
	testCfg.SteamAPIKey = "test-steam-api-key"
	testCfg.EmailInboundDomain = "mail.example.com"

	postgresDB := testhelper.ConnectTestDB(testCfg.DBDsn)
	testDB = postgresDB

	auth := sharedmocks.NewMockedAuthService(userID)
	logger := logging.NewNopLogger()

	testBooks = books.NewInner(
		context.Background(),
		auth,
		logger,
		testCfg,
		postgresDB,
		books.Clients{
			UniCat:           nil,
			Hardcover:        nil,
			ObjectStore:      booksobjectstore.NewFake(),
			WebFetch:         sharedmocks.NewMockWebFetchClient(),
			KoboStoreBaseURL: "",
			PublicAPIBaseURL: "",
		},
	)
	if err := testBooks.ApplyMigrations(context.Background(), postgresDB); err != nil {
		panic(err)
	}

	testFeeds = feeds.NewInner(
		context.Background(),
		auth,
		logger,
		testCfg,
		postgresDB,
		sharedmocks.NewMockWebFetchClient(),
		notifications.New(
			context.Background(),
			logger,
			mailer.New("", "", ""),
		),
		sharedrepos.NewAppUsersRepository(postgresDB),
	)
	if err := testFeeds.ApplyMigrations(context.Background(), postgresDB); err != nil {
		panic(err)
	}

	testGames = games.NewInner(
		context.Background(),
		auth,
		logger,
		testCfg,
		postgresDB,
		func(_ string) steam.Client { return fakeSteamClient{} },
	)
	if err := testGames.ApplyMigrations(context.Background(), postgresDB); err != nil {
		panic(err)
	}

	testToday = newTodayApps(auth, logger, postgresDB)

	testApp = dashboard.New(
		context.Background(),
		auth,
		logger,
		testCfg,
		postgresDB,
		testGames,
		testBooks,
		testFeeds,
		testToday,
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

// newTodayApps builds the apps the Today page reads, with their schemas.
func newTodayApps(
	auth sharedauth.Service,
	logger *slog.Logger,
	db *pgxpool.Pool,
) dashboard.TodayApps {
	ctx := context.Background()
	family := sharedrepos.NewFamilyRepository(db)
	sealer, err := crypto.New(testCfg.EncryptionKey)
	if err != nil {
		panic(err)
	}

	// Tests share one database; ensure the family tables exist.
	if _, err = db.Exec(ctx, `
		CREATE SCHEMA IF NOT EXISTS global;
		CREATE TABLE IF NOT EXISTS global.families (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE IF NOT EXISTS global.family_members (
			user_id TEXT PRIMARY KEY,
			family_id UUID NOT NULL REFERENCES global.families (id) ON DELETE CASCADE,
			joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			display_name TEXT NOT NULL DEFAULT ''
		)`); err != nil {
		panic(err)
	}

	// mealplans adopts tables from recipes, so recipes migrates first.
	if err = recipes.New(ctx, auth, logger, testCfg, db, family).
		ApplyMigrations(ctx, db); err != nil {
		panic(err)
	}

	today := dashboard.TodayApps{
		Books:        testBooks,
		Feeds:        testFeeds,
		MealPlans:    mealplans.New(ctx, auth, logger, testCfg, db, family),
		ShoppingList: shoppinglist.New(ctx, auth, logger, testCfg, db, family),
		Movies:       movies.New(ctx, auth, logger, testCfg, db, family),
		LearningPaths: learningpaths.New(
			ctx, auth, logger, testCfg, db, sealer, testBooks, testFeeds,
		),
		Factory: factory.New(ctx, auth, logger, testCfg, db),
	}
	for _, a := range []interface {
		ApplyMigrations(context.Context, *pgxpool.Pool) error
	}{
		today.MealPlans, today.ShoppingList, today.Movies,
		today.LearningPaths, today.Factory,
	} {
		if err = a.ApplyMigrations(ctx, db); err != nil {
			panic(err)
		}
	}
	return today
}
