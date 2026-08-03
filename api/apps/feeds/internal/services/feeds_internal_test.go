package services

import (
	"context"
	"log/slog"
	"testing"

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
		slog.Default(), nil, nil, mocks.NewMockWebFetchClient(), "",
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
