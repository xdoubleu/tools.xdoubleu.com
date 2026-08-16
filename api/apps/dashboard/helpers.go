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

func protoSharedFeeds(sharedFeeds []feeds.SharedFeed) []*dashboardv1.SharedFeed {
	proto := make([]*dashboardv1.SharedFeed, 0, len(sharedFeeds))
	for _, f := range sharedFeeds {
		proto = append(proto, &dashboardv1.SharedFeed{
			Title: f.Title,
			Url:   f.URL,
		})
	}
	return proto
}
