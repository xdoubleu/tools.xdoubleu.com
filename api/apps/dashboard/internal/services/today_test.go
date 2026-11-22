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
	book := services.ReadingBook{
		ID: "b1", Title: "Dune", Authors: nil, ProgressPercent: 0,
	}
	return []services.ReadingBook{book}, f.err
}

func (f *fakeSources) BuildTodayUnread(
	context.Context, string, int32,
) (int, []services.FeedItem, error) {
	return 2, []services.FeedItem{{ID: "i1", Title: "", FeedTitle: "", URL: ""}}, nil
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
	return []services.WatchingTitle{{ID: "w1", Title: "", HasNewSeason: false}}, nil
}

func (f *fakeSources) ListActivePaths(
	context.Context, string, int32,
) ([]services.LearningPath, error) {
	return []services.LearningPath{{ID: "p1", Title: ""}}, nil
}

func (f *fakeSources) ListAttentionTasks(
	context.Context, int,
) ([]services.AttentionTask, error) {
	return []services.AttentionTask{{Ref: "T-1", Title: "", Status: ""}}, nil
}

func adminUser() *sharedmodels.User {
	return &sharedmodels.User{
		ID: "u", Email: "", Role: sharedmodels.RoleAdmin, AppAccess: nil,
		WriteAccess: nil, PrincipalID: "", HasMFA: false, DisplayName: "",
	}
}

func allSources(f *fakeSources) services.TodaySources {
	return services.TodaySources{
		Reading: f, Feeds: f, Meals: f, Shopping: f,
		Watching: f, Learning: f, Attention: f,
	}
}

func TestToday_AdminGetsEveryBlock(t *testing.T) {
	f := &fakeSources{err: nil, gotLimit: 0}
	svc := services.NewTodayService(logging.NewNopLogger(), allSources(f))

	got := svc.Get(t.Context(), adminUser())

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
	f := &fakeSources{err: nil, gotLimit: 0}
	svc := services.NewTodayService(logging.NewNopLogger(), allSources(f))

	got := svc.Get(t.Context(), func() *sharedmodels.User {
		u := adminUser()
		u.Role, u.AppAccess = sharedmodels.RoleUser, []string{"movies"}
		return u
	}())

	assert.NotNil(t, got.Watching)
	assert.Nil(t, got.Reading)
	assert.Nil(t, got.Feeds)
	assert.Nil(t, got.Attention)
}

func TestToday_FailedBlockIsLeftUnset(t *testing.T) {
	f := &fakeSources{err: errors.New("boom"), gotLimit: 0}
	svc := services.NewTodayService(logging.NewNopLogger(), allSources(f))

	got := svc.Get(t.Context(), adminUser())

	assert.Nil(t, got.Reading)
	assert.NotNil(t, got.Feeds)
}

func TestToday_MissingSourceIsSkipped(t *testing.T) {
	svc := services.NewTodayService(logging.NewNopLogger(), services.TodaySources{
		Reading: nil, Feeds: nil, Meals: nil, Shopping: nil,
		Watching: nil, Learning: nil, Attention: nil,
	})

	got := svc.Get(t.Context(), adminUser())

	assert.Equal(t, new(services.Today), got)
}
