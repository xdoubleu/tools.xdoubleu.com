package feeds

import "context"

// UnhealthyFeed is a read-only projection of a feed failing to poll.
type UnhealthyFeed struct {
	Title               string
	URL                 string
	LastError           string
	ConsecutiveFailures int
}

// ListUnhealthy returns every failing feed across all users; used by the
// weekly digest job.
func (a *Feeds) ListUnhealthy(ctx context.Context) ([]UnhealthyFeed, error) {
	feeds, err := a.Services.Feeds.ListUnhealthy(ctx)
	if err != nil {
		return nil, err
	}

	unhealthy := make([]UnhealthyFeed, 0, len(feeds))
	for _, feed := range feeds {
		lastError := ""
		if feed.LastError != nil {
			lastError = *feed.LastError
		}
		unhealthy = append(unhealthy, UnhealthyFeed{
			Title:               feed.Title,
			URL:                 feed.URL,
			LastError:           lastError,
			ConsecutiveFailures: feed.ConsecutiveFailures,
		})
	}
	return unhealthy, nil
}
