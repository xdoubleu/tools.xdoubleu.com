package services

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/feeds/internal/mocks"
	"tools.xdoubleu.com/apps/feeds/internal/models"
)

func TestTitleOrDefault(t *testing.T) {
	cases := []struct {
		name     string
		title    string
		fallback string
		want     string
	}{
		{"non-blank title kept", "Real Title", "fallback", "Real Title"},
		{"empty title uses fallback", "", "fallback", "fallback"},
		{"whitespace-only title uses fallback", "   \t\n", "fallback", "fallback"},
		{"surrounding whitespace trimmed", "  Real Title  ", "fallback", "Real Title"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, titleOrDefault(c.title, c.fallback))
		})
	}
}

// TestBuildItemWhitespaceTitleFallsBackToCanonical proves an RSS item whose
// <title> is whitespace-only (not just "") ends up with the canonical URL as
// its title rather than a blank string (issue #763) once the linked page
// fetch also fails to yield a title.
func TestBuildItemWhitespaceTitleFallsBackToCanonical(t *testing.T) {
	s := NewFeedService(
		slog.Default(), nil, nil, mocks.NewMockWebFetchClient(), "", nil, nil, "",
	)
	//nolint:exhaustruct // only title/link/description are relevant here
	item := &gofeed.Item{
		Title:       "   ",
		Link:        "https://example.com/post",
		Description: "some description",
	}
	//nolint:exhaustruct // buildItem only reads ID off the feed
	feed := models.Feed{ID: uuid.New()}

	built, err := s.buildItem(context.Background(), feed, item, "guid-1")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/post", built.Title)
}

// TestIsFeedQuiet covers the issue #799 quiet-feed cadence heuristic.
func TestIsFeedQuiet(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	daysAgo := func(d int) time.Time { return now.Add(-time.Duration(d) * 24 * time.Hour) }

	cases := []struct {
		name  string
		times []time.Time
		want  bool
	}{
		{
			name:  "fewer than 3 items never quiet",
			times: []time.Time{daysAgo(100), daysAgo(50)},
			want:  false,
		},
		{
			name: "regular daily cadence, posted yesterday: not quiet",
			times: []time.Time{
				daysAgo(3), daysAgo(2), daysAgo(1),
			},
			want: false,
		},
		{
			name: "regular daily cadence, silent 10 days: quiet (avg*3 triggers)",
			times: []time.Time{
				daysAgo(13), daysAgo(12), daysAgo(11),
			},
			want: true,
		},
		{
			name: "sparse weekly cadence, 6 days silent: not quiet (avg*3 protects a slow feed)",
			times: []time.Time{
				daysAgo(20), daysAgo(13), daysAgo(6),
			},
			want: false,
		},
		{
			name: "single big gap where the 48h floor (not avg*3) is what triggers",
			times: []time.Time{
				// avg gap between the first two is 1h, so avg*3 = 3h — far
				// below the 48h floor; only the floor should catch this.
				now.Add(
					-72 * time.Hour,
				), now.Add(-71 * time.Hour), now.Add(-70 * time.Hour),
			},
			want: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, isFeedQuiet(c.times, now))
		})
	}
}
