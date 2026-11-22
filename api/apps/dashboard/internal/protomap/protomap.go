// Package protomap maps the dashboard domain models to their proto messages.
package protomap

import (
	"tools.xdoubleu.com/apps/dashboard/internal/services"
	dashboardv1 "tools.xdoubleu.com/gen/dashboard/v1"
)

// Today maps the Today summary to its wire message.
func Today(t *services.Today) *dashboardv1.GetTodayResponse {
	return &dashboardv1.GetTodayResponse{
		Reading:  reading(t.Reading),
		Feeds:    feeds(t.Feeds),
		Meals:    meals(t.Meals),
		Shopping: shopping(t.Shopping),
		Watching: watching(t.Watching),
		Learning: learning(t.Learning),
		Factory:  attention(t.Attention),
	}
}

//nolint:gosec // counts are bounded by the list sizes
func reading(l *services.List[services.ReadingBook]) *dashboardv1.TodayReading {
	if l == nil {
		return nil
	}
	block := &dashboardv1.TodayReading{}
	for _, b := range l.Items {
		block.Books = append(block.Books, &dashboardv1.TodayBook{
			Id:              b.ID,
			Title:           b.Title,
			Authors:         b.Authors,
			ProgressPercent: int32(b.ProgressPercent),
		})
	}
	return block
}

//nolint:gosec // counts are bounded by the list sizes
func feeds(f *services.TodayFeeds) *dashboardv1.TodayFeeds {
	if f == nil {
		return nil
	}
	block := &dashboardv1.TodayFeeds{UnreadCount: int32(f.UnreadCount)}
	for _, it := range f.Items {
		block.Items = append(block.Items, &dashboardv1.TodayFeedItem{
			Id:        it.ID,
			Title:     it.Title,
			FeedTitle: it.FeedTitle,
			Url:       it.URL,
		})
	}
	return block
}

func meals(l *services.List[services.Meal]) *dashboardv1.TodayMeals {
	if l == nil {
		return nil
	}
	block := &dashboardv1.TodayMeals{}
	for _, m := range l.Items {
		block.Meals = append(block.Meals, &dashboardv1.TodayMeal{
			Date: m.Date, Slot: m.Slot, Name: m.Name,
		})
	}
	return block
}

//nolint:gosec // counts are bounded by the list sizes
func shopping(n *int) *dashboardv1.TodayShopping {
	if n == nil {
		return nil
	}
	return &dashboardv1.TodayShopping{ItemCount: int32(*n)}
}

func watching(l *services.List[services.WatchingTitle]) *dashboardv1.TodayWatching {
	if l == nil {
		return nil
	}
	block := &dashboardv1.TodayWatching{}
	for _, w := range l.Items {
		block.Titles = append(block.Titles, &dashboardv1.TodayTitle{
			Id: w.ID, Title: w.Title, HasNewSeason: w.HasNewSeason,
		})
	}
	return block
}

func learning(l *services.List[services.LearningPath]) *dashboardv1.TodayLearning {
	if l == nil {
		return nil
	}
	block := &dashboardv1.TodayLearning{}
	for _, p := range l.Items {
		block.Paths = append(block.Paths, &dashboardv1.TodayPath{
			Id: p.ID, Title: p.Title,
		})
	}
	return block
}

func attention(l *services.List[services.AttentionTask]) *dashboardv1.TodayFactory {
	if l == nil {
		return nil
	}
	block := &dashboardv1.TodayFactory{}
	for _, task := range l.Items {
		block.Tasks = append(block.Tasks, &dashboardv1.TodayTask{
			Ref: task.Ref, Title: task.Title, Status: task.Status,
		})
	}
	return block
}
