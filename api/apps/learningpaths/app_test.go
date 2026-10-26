package learningpaths_test

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/books"
	booksobjectstore "tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/apps/feeds"
	"tools.xdoubleu.com/apps/learningpaths"
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
	testApp   *learningpaths.LearningPaths
	testBooks *books.Books
	testFeeds *feeds.Feeds
	testDB    postgres.DB
	testCfg   config.Config
	userID    = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"
)

func TestMain(m *testing.M) {
	testCfg = testhelper.NewTestConfig()
	testCfg.EmailInboundDomain = "mail.example.com"

	postgresDB := testhelper.ConnectTestDB(testCfg.DBDsn)
	testDB = postgresDB

	auth := sharedmocks.NewMockedAuthService(userID)
	logger := logging.NewNopLogger()

	testSealer, err := crypto.New(testCfg.EncryptionKey)
	if err != nil {
		panic(err)
	}

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
	if err = testBooks.ApplyMigrations(context.Background(), postgresDB); err != nil {
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
	if err = testFeeds.ApplyMigrations(context.Background(), postgresDB); err != nil {
		panic(err)
	}

	testApp = learningpaths.New(
		context.Background(),
		auth,
		logger,
		testCfg,
		postgresDB,
		testSealer,
		testBooks,
		testFeeds,
	)

	// Drop the schema so the rewritten migration is applied from scratch.
	if _, err = postgresDB.Exec(
		context.Background(),
		"DROP SCHEMA IF EXISTS learningpaths CASCADE",
	); err != nil {
		panic(err)
	}

	if err = testApp.ApplyMigrations(context.Background(), postgresDB); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

func getRoutes() http.Handler {
	return testhelper.BuildMux(testApp)
}

func TestGetDisplayName(t *testing.T) {
	assert.Equal(t, "Learning Paths", testApp.GetDisplayName())
}
