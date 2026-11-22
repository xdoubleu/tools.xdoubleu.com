package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/dashboard/internal/services"
	"tools.xdoubleu.com/internal/kit/logging"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

type fakeSources struct {
	err      error
	gotLimit int
}

func (f *fakeSources) ListCurrentlyReading(
	_ context.Context, _ string, limit int,
) ([]services.ReadingBook, error) {
	f.gotLimit = limit
	return []services.ReadingBook{{ID: "b1", Title: "Dune"}}, f.err
}

func (f *fakeSources) BuildTodayUnread(
	context.Context, string, int32,
) (int, []services.FeedItem, error) {
	return 2, []services.FeedItem{{ID: "i1"}}, nil
}

func (f *fakeSources) ListTodayMeals(
	context.Context, string, time.Time,
) ([]services.Meal, error) {
	return nil, nil
}

func (f *fakeSources) CountCustomListItems(context.Context, string) (int, error) {
	return 4, nil
}

func (f *fakeSources) ListWatching(
	context.Context, string, int32,
) ([]services.WatchingTitle, error) {
	return []services.WatchingTitle{{ID: "w1"}}, nil
}

func (f *fakeSources) ListActivePaths(
	context.Context, string, int32,
) ([]services.LearningPath, error) {
	return []services.LearningPath{{ID: "p1"}}, nil
}

func (f *fakeSources) ListAttentionTasks(
	context.Context, int,
) ([]services.AttentionTask, error) {
	return []services.AttentionTask{{Ref: "T-1"}}, nil
}

func allSources(f *fakeSources) services.TodaySources {
	return services.TodaySources{
		Reading: f, Feeds: f, Meals: f, Shopping: f,
		Watching: f, Learning: f, Attention: f,
	}
}

func TestToday_AdminGetsEveryBlock(t *testing.T) {
	f := &fakeSources{}
	svc := services.NewTodayService(logging.NewNopLogger(), allSources(f))

	got := svc.Get(t.Context(), &sharedmodels.User{ID: "u", Role: sharedmodels.RoleAdmin})

	require.NotNil(t, got.Reading)
	assert.Equal(t, "Dune", got.Reading.Items[0].Title)
	assert.Equal(t, services.TodayLimit, f.gotLimit)
	require.NotNil(t, got.Feeds)
	assert.Equal(t, 2, got.Feeds.UnreadCount)
	require.NotNil(t, got.Meals)
	assert.Empty(t, got.Meals.Items)
	require.NotNil(t, got.Shopping)
	assert.Equal(t, 4, *got.Shopping)
	assert.NotNil(t, got.Watching)
	assert.NotNil(t, got.Learning)
	assert.NotNil(t, got.Attention)
}

func TestToday_UserSeesOnlyGrantedApps(t *testing.T) {
	svc := services.NewTodayService(logging.NewNopLogger(), allSources(&fakeSources{}))

	got := svc.Get(t.Context(), &sharedmodels.User{
		ID: "u", Role: sharedmodels.RoleUser, AppAccess: []string{"movies"},
	})

	assert.NotNil(t, got.Watching)
	assert.Nil(t, got.Reading)
	assert.Nil(t, got.Feeds)
	assert.Nil(t, got.Attention)
}

func TestToday_FailedBlockIsLeftUnset(t *testing.T) {
	svc := services.NewTodayService(
		logging.NewNopLogger(), allSources(&fakeSources{err: errors.New("boom")}),
	)

	got := svc.Get(t.Context(), &sharedmodels.User{ID: "u", Role: sharedmodels.RoleAdmin})

	assert.Nil(t, got.Reading)
	assert.NotNil(t, got.Feeds)
}

func TestToday_MissingSourceIsSkipped(t *testing.T) {
	svc := services.NewTodayService(logging.NewNopLogger(), services.TodaySources{})

	got := svc.Get(t.Context(), &sharedmodels.User{ID: "u", Role: sharedmodels.RoleAdmin})

	assert.Equal(t, &services.Today{}, got)
}
