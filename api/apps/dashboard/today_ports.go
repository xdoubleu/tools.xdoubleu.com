package dashboard

import (
	"context"
	"time"

	"tools.xdoubleu.com/apps/dashboard/internal/services"
)

// Adapters from each app's exported Today API to the ports the Today service
// declares.

type readingPort struct{ TodayApps }

func (p readingPort) ListCurrentlyReading(
	ctx context.Context, userID string, limit int,
) ([]services.ReadingBook, error) {
	list, err := p.Books.ListCurrentlyReading(ctx, userID, limit)
	out := make([]services.ReadingBook, 0, len(list))
	for _, b := range list {
		out = append(out, services.ReadingBook(b))
	}
	return out, err
}

type feedsPort struct{ TodayApps }

func (p feedsPort) BuildTodayUnread(
	ctx context.Context, userID string, limit int32,
) (int, []services.FeedItem, error) {
	count, items, err := p.Feeds.BuildTodayUnread(ctx, userID, limit)
	out := make([]services.FeedItem, 0, len(items))
	for _, it := range items {
		out = append(out, services.FeedItem(it))
	}
	return count, out, err
}

type mealsPort struct{ TodayApps }

func (p mealsPort) ListTodayMeals(
	ctx context.Context, userID string, today time.Time,
) ([]services.Meal, error) {
	meals, err := p.MealPlans.ListTodayMeals(ctx, userID, today)
	out := make([]services.Meal, 0, len(meals))
	for _, m := range meals {
		out = append(out, services.Meal(m))
	}
	return out, err
}

type shoppingPort struct{ TodayApps }

func (p shoppingPort) CountCustomListItems(
	ctx context.Context, userID string,
) (int, error) {
	return p.ShoppingList.CountCustomListItems(ctx, userID)
}

type watchingPort struct{ TodayApps }

func (p watchingPort) ListWatching(
	ctx context.Context, userID string, limit int32,
) ([]services.WatchingTitle, error) {
	titles, err := p.Movies.ListWatching(ctx, userID, limit)
	out := make([]services.WatchingTitle, 0, len(titles))
	for _, t := range titles {
		out = append(out, services.WatchingTitle(t))
	}
	return out, err
}

type learningPort struct{ TodayApps }

func (p learningPort) ListActivePaths(
	ctx context.Context, userID string, limit int32,
) ([]services.LearningPath, error) {
	paths, err := p.LearningPaths.ListActivePaths(ctx, userID, limit)
	out := make([]services.LearningPath, 0, len(paths))
	for _, lp := range paths {
		out = append(out, services.LearningPath(lp))
	}
	return out, err
}

type attentionPort struct{ TodayApps }

func (p attentionPort) ListAttentionTasks(
	ctx context.Context, limit int,
) ([]services.AttentionTask, error) {
	tasks, err := p.Factory.ListAttentionTasks(ctx, limit)
	out := make([]services.AttentionTask, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, services.AttentionTask(t))
	}
	return out, err
}

func todaySources(t TodayApps) services.TodaySources {
	return services.TodaySources{
		Reading:   readingPort{t},
		Feeds:     feedsPort{t},
		Meals:     mealsPort{t},
		Shopping:  shoppingPort{t},
		Watching:  watchingPort{t},
		Learning:  learningPort{t},
		Attention: attentionPort{t},
	}
}
