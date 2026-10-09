package podcasts_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/podcasts/internal/models"
	"tools.xdoubleu.com/apps/podcasts/internal/repositories"
	"tools.xdoubleu.com/internal/database/postgres"
)

// countingDB wraps the shared test DB, counting Exec round trips.
type countingDB struct {
	postgres.DB
	execs int
}

func (c *countingDB) Exec(
	ctx context.Context,
	sql string,
	args ...any,
) (pgconn.CommandTag, error) {
	c.execs++
	return c.DB.Exec(ctx, sql, args...)
}

// insertShow seeds a favourite whose episodes can be upserted against.
func insertShow(t *testing.T) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := testDB.Exec(context.Background(), `
		INSERT INTO podcasts.shows (id, user_id, itunes_id, title, feed_url)
		VALUES ($1, $2, $3, $4, $5)`,
		id, "batch-test-user", int64(uuid.New().ID()), "Batch Show",
		"https://feeds.example/batch")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			"DELETE FROM podcasts.shows WHERE id = $1", id)
	})
	return id
}

func episode(showID uuid.UUID, guid, title string) models.Episode {
	return models.Episode{
		ID:              uuid.Nil,
		ShowID:          showID,
		GUID:            guid,
		Title:           title,
		Summary:         "",
		Link:            "",
		AudioURL:        "",
		DurationSeconds: nil,
		PublishedAt:     nil,
	}
}

func TestEpisodesUpsert_BatchesAllEpisodesInOneStatement(t *testing.T) {
	db := &countingDB{DB: testDB, execs: 0}
	repo := repositories.New(db).Episodes
	showID := insertShow(t)

	eps := make([]models.Episode, 100)
	for i := range eps {
		eps[i] = episode(showID, fmt.Sprintf("g-%d", i), fmt.Sprintf("Episode %d", i))
	}

	require.NoError(t, repo.Upsert(context.Background(), showID, eps))
	assert.Equal(t, 1, db.execs, "one multi-row upsert, not one per episode")

	var n int
	require.NoError(t, testDB.QueryRow(context.Background(),
		"SELECT count(*) FROM podcasts.episodes WHERE show_id = $1", showID).Scan(&n))
	assert.Equal(t, len(eps), n)
}

func TestEpisodesUpsert_RepeatedGUIDWithinBatchKeepsLast(t *testing.T) {
	db := &countingDB{DB: testDB, execs: 0}
	repo := repositories.New(db).Episodes
	showID := insertShow(t)

	require.NoError(t, repo.Upsert(context.Background(), showID, []models.Episode{
		episode(showID, "dup", "first"),
		episode(showID, "dup", "second"),
	}))
	assert.Equal(t, 1, db.execs)

	var n int
	require.NoError(t, testDB.QueryRow(context.Background(),
		"SELECT count(*) FROM podcasts.episodes WHERE show_id = $1 AND guid = 'dup'",
		showID).Scan(&n))
	assert.Equal(t, 1, n, "the duplicate GUID resolves to a single row")

	var title string
	require.NoError(t, testDB.QueryRow(context.Background(),
		"SELECT title FROM podcasts.episodes WHERE show_id = $1 AND guid = 'dup'",
		showID).Scan(&title))
	assert.Equal(t, "second", title, "the last occurrence wins")
}
