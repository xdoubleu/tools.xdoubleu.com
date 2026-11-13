package dashboard

import (
	"context"
	"slices"
	"time"

	"connectrpc.com/connect/v2"
	"golang.org/x/sync/errgroup"

	"tools.xdoubleu.com/apps/books"
	"tools.xdoubleu.com/apps/factory"
	"tools.xdoubleu.com/apps/feeds"
	"tools.xdoubleu.com/apps/learningpaths"
	"tools.xdoubleu.com/apps/mealplans"
	"tools.xdoubleu.com/apps/movies"
	"tools.xdoubleu.com/apps/shoppinglist"
	dashboardv1 "tools.xdoubleu.com/gen/dashboard/v1"
	"tools.xdoubleu.com/gen/dashboard/v1/dashboardv1connect"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// todayLimit caps each list on the Today page.
const todayLimit = 3

type todayConnectHandler struct {
	app *Dashboard
}

var _ dashboardv1connect.TodayServiceHandler = (*todayConnectHandler)(nil)

func todayUser(ctx context.Context) *sharedmodels.User {
	return contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
}

func canUse(user *sharedmodels.User, appName string) bool {
	return user.Role == sharedmodels.RoleAdmin ||
		slices.Contains(user.AppAccess, appName)
}

// GetToday loads one block per app the caller can use, concurrently. A block
// that fails is logged and left unset so one slow or broken app can't blank
// the page.
func (h *todayConnectHandler) GetToday(
	ctx context.Context,
	_ *dashboardv1.GetTodayRequest,
) (*dashboardv1.GetTodayResponse, error) {
	user := todayUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated, "user not authenticated",
		)
	}

	resp := &dashboardv1.GetTodayResponse{}
	var g errgroup.Group
	for _, b := range todayBlocks(h.app.today, user, resp, time.Now()) {
		if !canUse(user, b.app) {
			continue
		}
		g.Go(func() error {
			if err := b.load(ctx); err != nil {
				h.app.Logger.ErrorContext(ctx, "today block failed",
					"app", b.app, "userID", user.ID, "error", err)
			}
			return nil
		})
	}
	_ = g.Wait()
	return resp, nil
}

type todayBlock struct {
	app  string
	load func(ctx context.Context) error
}

// todayBlocks returns the loaders that fill resp; each writes its own field
// only after its data has loaded.
func todayBlocks(
	t TodayApps,
	user *sharedmodels.User,
	resp *dashboardv1.GetTodayResponse,
	now time.Time,
) []todayBlock {
	return []todayBlock{
		{"books", func(ctx context.Context) error {
			return loadReading(ctx, t.Books, user.ID, resp)
		}},
		{"feeds", func(ctx context.Context) error {
			return loadFeeds(ctx, t.Feeds, user.ID, resp)
		}},
		{"mealplans", func(ctx context.Context) error {
			return loadMeals(ctx, t.MealPlans, user.ID, now, resp)
		}},
		{"shoppinglist", func(ctx context.Context) error {
			return loadShopping(ctx, t.ShoppingList, user.ID, resp)
		}},
		{"movies", func(ctx context.Context) error {
			return loadWatching(ctx, t.Movies, user.ID, resp)
		}},
		{"learningpaths", func(ctx context.Context) error {
			return loadLearning(ctx, t.LearningPaths, user.ID, resp)
		}},
		{"factory", func(ctx context.Context) error {
			return loadFactory(ctx, t.Factory, resp)
		}},
	}
}

func loadReading(
	ctx context.Context,
	app *books.Books,
	userID string,
	resp *dashboardv1.GetTodayResponse,
) error {
	list, err := app.ListCurrentlyReading(ctx, userID, todayLimit)
	if err != nil {
		return err
	}
	block := &dashboardv1.TodayReading{}
	for _, b := range list {
		block.Books = append(block.Books, &dashboardv1.TodayBook{
			Id:              b.ID,
			Title:           b.Title,
			Authors:         b.Authors,
			ProgressPercent: int32(b.ProgressPercent), //nolint:gosec // 0-100
		})
	}
	resp.Reading = block
	return nil
}

func loadFeeds(
	ctx context.Context,
	app *feeds.Feeds,
	userID string,
	resp *dashboardv1.GetTodayResponse,
) error {
	count, items, err := app.BuildTodayUnread(ctx, userID, todayLimit)
	if err != nil {
		return err
	}
	block := &dashboardv1.TodayFeeds{
		UnreadCount: int32(count), //nolint:gosec // bounded by the item count
	}
	for _, it := range items {
		block.Items = append(block.Items, &dashboardv1.TodayFeedItem{
			Id:        it.ID,
			Title:     it.Title,
			FeedTitle: it.FeedTitle,
			Url:       it.URL,
		})
	}
	resp.Feeds = block
	return nil
}

func loadMeals(
	ctx context.Context,
	app *mealplans.MealPlans,
	userID string,
	now time.Time,
	resp *dashboardv1.GetTodayResponse,
) error {
	meals, err := app.ListTodayMeals(ctx, userID, now)
	if err != nil {
		return err
	}
	block := &dashboardv1.TodayMeals{}
	for _, m := range meals {
		block.Meals = append(block.Meals, &dashboardv1.TodayMeal{
			Date: m.Date, Slot: m.Slot, Name: m.Name,
		})
	}
	resp.Meals = block
	return nil
}

func loadShopping(
	ctx context.Context,
	app *shoppinglist.ShoppingList,
	userID string,
	resp *dashboardv1.GetTodayResponse,
) error {
	n, err := app.CountCustomListItems(ctx, userID)
	if err != nil {
		return err
	}
	resp.Shopping = &dashboardv1.TodayShopping{
		ItemCount: int32(n), //nolint:gosec // bounded by the list size
	}
	return nil
}

func loadWatching(
	ctx context.Context,
	app *movies.Movies,
	userID string,
	resp *dashboardv1.GetTodayResponse,
) error {
	titles, err := app.ListWatching(ctx, userID, todayLimit)
	if err != nil {
		return err
	}
	block := &dashboardv1.TodayWatching{}
	for _, w := range titles {
		block.Titles = append(block.Titles, &dashboardv1.TodayTitle{
			Id: w.ID, Title: w.Title, HasNewSeason: w.HasNewSeason,
		})
	}
	resp.Watching = block
	return nil
}

func loadLearning(
	ctx context.Context,
	app *learningpaths.LearningPaths,
	userID string,
	resp *dashboardv1.GetTodayResponse,
) error {
	paths, err := app.ListActivePaths(ctx, userID, todayLimit)
	if err != nil {
		return err
	}
	block := &dashboardv1.TodayLearning{}
	for _, p := range paths {
		block.Paths = append(block.Paths, &dashboardv1.TodayPath{
			Id: p.ID, Title: p.Title,
		})
	}
	resp.Learning = block
	return nil
}

func loadFactory(
	ctx context.Context,
	app *factory.Factory,
	resp *dashboardv1.GetTodayResponse,
) error {
	tasks, err := app.ListAttentionTasks(ctx, todayLimit)
	if err != nil {
		return err
	}
	block := &dashboardv1.TodayFactory{}
	for _, task := range tasks {
		block.Tasks = append(block.Tasks, &dashboardv1.TodayTask{
			Ref: task.Ref, Title: task.Title, Status: task.Status,
		})
	}
	resp.Factory = block
	return nil
}
