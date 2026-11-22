// Package protomap maps the dashboard domain models to their proto messages.
package protomap

import (
	"tools.xdoubleu.com/apps/dashboard/internal/services"
	dashboardv1 "tools.xdoubleu.com/gen/dashboard/v1"
)

// Today maps the Today summary to its wire message.
//
//nolint:gosec // counts are bounded by the list sizes
func Today(t *services.Today) *dashboardv1.GetTodayResponse {
	resp := &dashboardv1.GetTodayResponse{}
	if t.Reading != nil {
		block := &dashboardv1.TodayReading{}
		for _, b := range t.Reading.Items {
			block.Books = append(block.Books, &dashboardv1.TodayBook{
				Id:              b.ID,
				Title:           b.Title,
				Authors:         b.Authors,
				ProgressPercent: int32(b.ProgressPercent),
			})
		}
		resp.Reading = block
	}
	if t.Feeds != nil {
		block := &dashboardv1.TodayFeeds{UnreadCount: int32(t.Feeds.UnreadCount)}
		for _, it := range t.Feeds.Items {
			block.Items = append(block.Items, &dashboardv1.TodayFeedItem{
				Id:        it.ID,
				Title:     it.Title,
				FeedTitle: it.FeedTitle,
				Url:       it.URL,
			})
		}
		resp.Feeds = block
	}
	if t.Meals != nil {
		block := &dashboardv1.TodayMeals{}
		for _, m := range t.Meals.Items {
			block.Meals = append(block.Meals, &dashboardv1.TodayMeal{
				Date: m.Date, Slot: m.Slot, Name: m.Name,
			})
		}
		resp.Meals = block
	}
	if t.Shopping != nil {
		resp.Shopping = &dashboardv1.TodayShopping{ItemCount: int32(*t.Shopping)}
	}
	if t.Watching != nil {
		block := &dashboardv1.TodayWatching{}
		for _, w := range t.Watching.Items {
			block.Titles = append(block.Titles, &dashboardv1.TodayTitle{
				Id: w.ID, Title: w.Title, HasNewSeason: w.HasNewSeason,
			})
		}
		resp.Watching = block
	}
	if t.Learning != nil {
		block := &dashboardv1.TodayLearning{}
		for _, p := range t.Learning.Items {
			block.Paths = append(block.Paths, &dashboardv1.TodayPath{
				Id: p.ID, Title: p.Title,
			})
		}
		resp.Learning = block
	}
	if t.Attention != nil {
		block := &dashboardv1.TodayFactory{}
		for _, task := range t.Attention.Items {
			block.Tasks = append(block.Tasks, &dashboardv1.TodayTask{
				Ref: task.Ref, Title: task.Title, Status: task.Status,
			})
		}
		resp.Factory = block
	}
	return resp
}
