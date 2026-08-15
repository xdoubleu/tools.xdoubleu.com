package dashboard

import (
	"time"

	"tools.xdoubleu.com/apps/feeds"
	dashboardv1 "tools.xdoubleu.com/gen/dashboard/v1"
)

// dateFormat matches the format games/books use for their own progress-chart
// date range params (apps/games/books' ProgressDateFormat).
const dateFormat = "2006-01-02"

func parseDateRangeFromStrings(dateStart, dateEnd string) (time.Time, time.Time) {
	end := time.Now()
	start := end.AddDate(-1, 0, 0)

	if dateStart != "" {
		if t, err := time.Parse(dateFormat, dateStart); err == nil {
			start = t
		}
	}
	if dateEnd != "" {
		if t, err := time.Parse(dateFormat, dateEnd); err == nil {
			end = t
		}
	}
	return start, end
}

func protoFeedsSummary(summary *feeds.Summary) *dashboardv1.FeedsSummary {
	items := make([]*dashboardv1.FeedsSummaryItem, 0, len(summary.Items))
	for _, item := range summary.Items {
		items = append(items, &dashboardv1.FeedsSummaryItem{
			Title:       item.Title,
			SourceUrl:   item.SourceURL,
			PublishedAt: item.PublishedAt.Format(time.RFC3339),
		})
	}

	//nolint:gosec // unread_count is a small domain count, never near int32 overflow
	return &dashboardv1.FeedsSummary{
		UnreadCount: int32(summary.UnreadCount),
		Items:       items,
	}
}
