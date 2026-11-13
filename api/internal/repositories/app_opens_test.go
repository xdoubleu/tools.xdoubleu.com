package repositories_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/database/postgres"
	"tools.xdoubleu.com/internal/kit/testhelper"
	"tools.xdoubleu.com/internal/repositories"
)

type appOpensEnv struct {
	repo   *repositories.AppUsersRepository
	db     postgres.DB
	userID string
}

func newAppOpensEnv(t *testing.T) appOpensEnv {
	t.Helper()
	pool := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(pool.Close)
	id := uuid.NewString()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM global.app_opens WHERE user_id = $1`, id)
	})
	return appOpensEnv{
		repo:   repositories.NewAppUsersRepository(pool),
		db:     pool,
		userID: id,
	}
}

// seed writes `count` opens of app `daysAgo` days back.
func (e appOpensEnv) seed(t *testing.T, app string, daysAgo, count int) {
	t.Helper()
	_, err := e.db.Exec(context.Background(), `
		INSERT INTO global.app_opens (user_id, day, app, count)
		VALUES ($1, CURRENT_DATE - $2::int, $3, $4)
	`, e.userID, daysAgo, app, count)
	require.NoError(t, err)
}

func (e appOpensEnv) rows(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, e.db.QueryRow(context.Background(),
		`SELECT count(*) FROM global.app_opens WHERE user_id = $1`, e.userID,
	).Scan(&n))
	return n
}

func TestMostOpenedApps_RanksWholeDaysInTheWindow(t *testing.T) {
	e := newAppOpensEnv(t)
	e.seed(t, "recipes", 1, 5)
	e.seed(t, "recipes", 2, 4)
	e.seed(t, "mealplans", 3, 6)
	e.seed(t, "feeds", 10, 2)

	apps, err := e.repo.MostOpenedApps(context.Background(), e.userID)

	require.NoError(t, err)
	assert.Equal(t, []string{"recipes", "mealplans", "feeds"}, apps)
}

func TestMostOpenedApps_LeavesOutTodayAndOldDays(t *testing.T) {
	e := newAppOpensEnv(t)
	e.seed(t, "today", 0, 99)
	e.seed(t, "edge", 30, 3)
	e.seed(t, "ancient", 31, 99)

	apps, err := e.repo.MostOpenedApps(context.Background(), e.userID)

	require.NoError(t, err)
	assert.Equal(t, []string{"edge"}, apps)
}

func TestMostOpenedApps_BreaksTiesByNameAndCapsTheList(t *testing.T) {
	e := newAppOpensEnv(t)
	for _, app := range []string{"i", "h", "g", "f", "e", "d", "c", "b", "a"} {
		e.seed(t, app, 1, 1)
	}

	apps, err := e.repo.MostOpenedApps(context.Background(), e.userID)

	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c", "d", "e", "f", "g", "h"}, apps)
}

func TestMostOpenedApps_EmptyWithoutHistory(t *testing.T) {
	e := newAppOpensEnv(t)

	apps, err := e.repo.MostOpenedApps(context.Background(), e.userID)

	require.NoError(t, err)
	assert.NotNil(t, apps)
	assert.Empty(t, apps)
}

func TestRecordAppOpen_CountsPerDayAndApp(t *testing.T) {
	e := newAppOpensEnv(t)
	ctx := context.Background()

	require.NoError(t, e.repo.RecordAppOpen(ctx, e.userID, "books"))
	require.NoError(t, e.repo.RecordAppOpen(ctx, e.userID, "books"))
	require.NoError(t, e.repo.RecordAppOpen(ctx, e.userID, "feeds"))

	var books, feeds int
	require.NoError(t, e.db.QueryRow(ctx, `
		SELECT count FROM global.app_opens
		WHERE user_id = $1 AND app = 'books' AND day = CURRENT_DATE`, e.userID,
	).Scan(&books))
	require.NoError(t, e.db.QueryRow(ctx, `
		SELECT count FROM global.app_opens
		WHERE user_id = $1 AND app = 'feeds' AND day = CURRENT_DATE`, e.userID,
	).Scan(&feeds))
	assert.Equal(t, 2, books)
	assert.Equal(t, 1, feeds)
}

func TestRecordAppOpen_DropsThisUsersOldRowsOnly(t *testing.T) {
	e := newAppOpensEnv(t)
	other := newAppOpensEnv(t)
	e.seed(t, "kept", 90, 1)
	e.seed(t, "gone", 91, 1)
	other.seed(t, "elsewhere", 200, 1)

	require.NoError(t, e.repo.RecordAppOpen(context.Background(), e.userID, "books"))

	assert.Equal(t, 2, e.rows(t), "the 91-day-old row is deleted, the rest stay")
	assert.Equal(t, 1, other.rows(t), "another user's old rows are untouched")
}
