package services

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"golang.org/x/sync/errgroup"

	sharedmodels "tools.xdoubleu.com/internal/models"
)

// TodaySources are the per-app ports the Today page reads. A nil source is
// skipped.
type TodaySources struct {
	Reading   ReadingSource
	Feeds     FeedsSource
	Meals     MealsSource
	Shopping  ShoppingSource
	Watching  WatchingSource
	Learning  LearningSource
	Attention AttentionSource
}

// Today holds one block per app the caller can use; a block is nil when the
// caller lacks access or the app failed to load.
type Today struct {
	Reading   *List[ReadingBook]
	Feeds     *TodayFeeds
	Meals     *List[Meal]
	Shopping  *int
	Watching  *List[WatchingTitle]
	Learning  *List[LearningPath]
	Attention *List[AttentionTask]
}

// List is a block of entries.
type List[T any] struct{ Items []T }

// TodayFeeds is the feeds block.
type TodayFeeds struct {
	UnreadCount int
	Items       []FeedItem
}

// TodayService assembles the Today summary.
type TodayService struct {
	logger  *slog.Logger
	sources TodaySources
	now     func() time.Time
}

// NewTodayService returns a TodayService over the given sources.
func NewTodayService(logger *slog.Logger, sources TodaySources) *TodayService {
	return &TodayService{logger: logger, sources: sources, now: time.Now}
}

// CanUse reports whether user may use the named app.
func CanUse(user *sharedmodels.User, appName string) bool {
	return user.Role == sharedmodels.RoleAdmin ||
		slices.Contains(user.AppAccess, appName)
}

type todayBlock struct {
	app  string
	load func(ctx context.Context) error
}

// Get loads one block per app user can use, concurrently. A block that fails
// is logged and left unset so one slow or broken app can't blank the page.
func (s *TodayService) Get(ctx context.Context, user *sharedmodels.User) *Today {
	out := &Today{}
	var g errgroup.Group
	for _, b := range s.blocks(user, out, s.now()) {
		if !CanUse(user, b.app) {
			continue
		}
		g.Go(func() error {
			if err := b.load(ctx); err != nil {
				s.logger.ErrorContext(ctx, "today block failed",
					"app", b.app, "userID", user.ID, "error", err)
			}
			return nil
		})
	}
	_ = g.Wait()
	return out
}

// blocks returns the loaders that fill out; each writes its own fields only
// after its data has loaded.
func (s *TodayService) blocks(
	user *sharedmodels.User,
	out *Today,
	now time.Time,
) []todayBlock {
	src := s.sources
	var blocks []todayBlock
	add := func(app string, present bool, load func(ctx context.Context) error) {
		if present {
			blocks = append(blocks, todayBlock{app, load})
		}
	}

	add("books", src.Reading != nil, func(ctx context.Context) error {
		books, err := src.Reading.ListCurrentlyReading(ctx, user.ID, TodayLimit)
		if err == nil {
			out.Reading = &List[ReadingBook]{Items: books}
		}
		return err
	})
	add("feeds", src.Feeds != nil, func(ctx context.Context) error {
		count, items, err := src.Feeds.BuildTodayUnread(ctx, user.ID, TodayLimit)
		if err == nil {
			out.Feeds = &TodayFeeds{UnreadCount: count, Items: items}
		}
		return err
	})
	add("mealplans", src.Meals != nil, func(ctx context.Context) error {
		meals, err := src.Meals.ListTodayMeals(ctx, user.ID, now)
		if err == nil {
			out.Meals = &List[Meal]{Items: meals}
		}
		return err
	})
	add("shoppinglist", src.Shopping != nil, func(ctx context.Context) error {
		n, err := src.Shopping.CountCustomListItems(ctx, user.ID)
		if err == nil {
			out.Shopping = &n
		}
		return err
	})
	add("movies", src.Watching != nil, func(ctx context.Context) error {
		titles, err := src.Watching.ListWatching(ctx, user.ID, TodayLimit)
		if err == nil {
			out.Watching = &List[WatchingTitle]{Items: titles}
		}
		return err
	})
	add("learningpaths", src.Learning != nil, func(ctx context.Context) error {
		paths, err := src.Learning.ListActivePaths(ctx, user.ID, TodayLimit)
		if err == nil {
			out.Learning = &List[LearningPath]{Items: paths}
		}
		return err
	})
	add("factory", src.Attention != nil, func(ctx context.Context) error {
		tasks, err := src.Attention.ListAttentionTasks(ctx, TodayLimit)
		if err == nil {
			out.Attention = &List[AttentionTask]{Items: tasks}
		}
		return err
	})
	return blocks
}
