package feeds_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/feeds/internal/repositories"
	"tools.xdoubleu.com/apps/feeds/internal/services"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mocks"
)

// itemInsertCountingDB counts item-insert statements, so a test can assert a
// poll stored its rows through the batched path rather than one INSERT per
// item (the poll-feeds N+1 Sentry keeps flagging).
type itemInsertCountingDB struct {
	postgres.DB
	itemInserts atomic.Int64
}

func (c *itemInsertCountingDB) Exec(
	ctx context.Context,
	sql string,
	args ...any,
) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "INSERT INTO feeds.items") {
		c.itemInserts.Add(1)
	}
	return c.DB.Exec(ctx, sql, args...)
}

// A scrape poll with several new posts must store them all in one batched
// INSERT; per-item INSERTs are the N+1 pattern Sentry flags as poll-feeds.
func TestScrapePollStoresItemsInOneBatchInsert(t *testing.T) {
	//nolint:exhaustruct // embedded DB delegates; itemInserts counts from zero
	counting := &itemInsertCountingDB{DB: testDB}
	webFetch := mocks.NewMockWebFetchClient()

	base := "https://scrape-batch-" + uuid.NewString() + ".example.com"
	const posts = 4
	list := &strings.Builder{}
	list.WriteString("<html><body><main>")
	for i := range posts {
		slug := fmt.Sprintf("/posts/post-%d", i)
		title := fmt.Sprintf("Batch post number %d with a long enough title", i)
		fmt.Fprintf(list, `<a href="%s">%s</a>`, slug, title)
		webFetch.SetHTML(base+slug, articlePageHTML(title))
	}
	list.WriteString("</main></body></html>")
	webFetch.SetHTML(base, list.String())

	feedID := seedScrapeFeed(t, base)
	repos := repositories.New(counting)
	svc := services.NewFeedService(
		logging.NewNopLogger(), nil,
		repos.Feeds, repos.Items, repos.FilterRules,
		webFetch, "",
		nil, appUsersRepo, "",
	)

	counting.itemInserts.Store(0)
	ingested, err := svc.Refresh(context.Background(), userID, feedID)
	require.NoError(t, err)
	assert.Equal(t, posts, ingested)

	assert.Equal(t, int64(1), counting.itemInserts.Load(),
		"a scrape poll must store every new post in a single INSERT")

	var stored int
	require.NoError(t, testDB.QueryRow(context.Background(), `
		SELECT count(*) FROM feeds.items WHERE feed_id = $1`, feedID).Scan(&stored))
	assert.Equal(t, posts, stored)
}

// A post whose content fetch fails is stored as a batched seen-error marker,
// never retried — still one INSERT for the whole poll.
func TestScrapePollStoresFetchFailuresAsBatchedSeenMarkers(t *testing.T) {
	//nolint:exhaustruct // embedded DB delegates; itemInserts counts from zero
	counting := &itemInsertCountingDB{DB: testDB}
	webFetch := mocks.NewMockWebFetchClient()

	base := "https://scrape-marker-" + uuid.NewString() + ".example.com"
	good := fmt.Sprintf("%s/posts/good-post", base)
	bad := fmt.Sprintf("%s/posts/bad-post", base)
	webFetch.SetHTML(base, `<html><body><main>
		<a href="/posts/good-post">A good post with a long enough title</a>
		<a href="/posts/bad-post">A bad post with a long enough title</a>
	</main></body></html>`)
	webFetch.SetHTML(good, articlePageHTML("Good Post"))
	webFetch.Errs[bad] = errors.New("content fetch failed")

	feedID := seedScrapeFeed(t, base)
	repos := repositories.New(counting)
	svc := services.NewFeedService(
		logging.NewNopLogger(), nil,
		repos.Feeds, repos.Items, repos.FilterRules,
		webFetch, "",
		nil, appUsersRepo, "",
	)

	counting.itemInserts.Store(0)
	ingested, err := svc.Refresh(context.Background(), userID, feedID)
	require.NoError(t, err)
	assert.Equal(t, 1, ingested)

	assert.Equal(t, int64(1), counting.itemInserts.Load(),
		"seen-error markers must ride in the same batched INSERT")

	var n int
	require.NoError(t, testDB.QueryRow(context.Background(), `
		SELECT count(*) FROM feeds.items
		WHERE feed_id = $1 AND guid = $2 AND ingest_error = 'content fetch failed'`,
		feedID, bad).Scan(&n))
	assert.Equal(t, 1, n)
}

// seedScrapeFeed inserts a scrape feed row for userID, removed on cleanup.
func seedScrapeFeed(t *testing.T, url string) uuid.UUID {
	t.Helper()
	feedID := uuid.New()
	_, err := testDB.Exec(context.Background(), `
		INSERT INTO feeds.feeds (id, user_id, url, title, source_type)
		VALUES ($1, $2, $3, 'Scrape Batch Blog', 'scrape')
	`, feedID, userID, url)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			"DELETE FROM feeds.feeds WHERE id = $1", feedID)
	})
	return feedID
}
