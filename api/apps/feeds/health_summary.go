package feeds

import "context"

// UnhealthyFeed is a lightweight, read-only projection of one feed
// currently failing to poll, for the weekly digest job (issue #1014).
type UnhealthyFeed struct {
	Title               string
	URL                 string
	LastError           string
	ConsecutiveFailures int
}

// ListUnhealthy returns every feed currently failing to poll, across all
// users. It is the only exported entry point feeds' internal model is
// reached through from outside this package for this purpose — the
// weekly digest job (api/internal/observability/jobs) calls this instead
// of querying feeds.feeds directly.
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
