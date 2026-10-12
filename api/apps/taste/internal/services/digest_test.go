package services_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/taste/internal/services"
	"tools.xdoubleu.com/internal/consumption"
)

func TestDigest(t *testing.T) {
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	recent := now.AddDate(0, -1, 0)
	old := now.AddDate(-2, 0, 0)
	events := []consumption.Event{
		{
			Domain: consumption.DomainBooks, ItemID: "1", Title: "Dune",
			Creator: "Frank Herbert", Tags: []string{"sci-fi"}, At: old,
			Signal: consumption.StrongPositive, Reason: "rated 5★",
		},
		{
			Domain: consumption.DomainBooks, ItemID: "2", Title: "Okay Book",
			Creator: "", Tags: nil, At: recent,
			Signal: consumption.Positive, Reason: "finished",
		},
		{
			Domain: consumption.DomainGames, ItemID: "3", Title: "Bounced",
			Creator: "", Tags: nil, At: old,
			Signal: consumption.Negative, Reason: "under an hour played",
		},
		{
			Domain: consumption.DomainGames, ItemID: "5", Title: "Hated",
			Creator: "", Tags: nil, At: time.Time{},
			Signal: consumption.StrongNegative, Reason: "rated 1★",
		},
		{
			Domain: consumption.DomainGames, ItemID: "4", Title: "Meh",
			Creator: "", Tags: nil, At: recent,
			Signal: consumption.Neutral, Reason: "played",
		},
	}

	got := services.Digest(events, now)

	assert.True(t, strings.HasPrefix(got, "Today is 2026-10-12.\n"))
	assert.Contains(t, got, "## books: 2 consumed, 2 liked, 0 disliked\nLiked most:\n"+
		"- Dune — Frank Herbert [sci-fi] (books, +2 rated 5★, 2024-10)\n"+
		"- Okay Book (books, +1 finished, 2026-09)\n")
	// Strongest first, though Hated is undated and Bounced dated.
	assert.Contains(t, got, "## games: 3 consumed, 0 liked, 2 disliked\n"+
		"Disliked:\n- Hated (games, -2 rated 1★)\n"+
		"- Bounced (games, -1 under an hour played, 2024-10)\n")
	assert.NotContains(t, got, "## feeds")
	// Same date, so by title; undated events never count as recent.
	assert.Contains(t, got, "## Last 90 days:\n- Meh (games, +0 played, 2026-09)\n"+
		"- Okay Book (books, +1 finished, 2026-09)\n\n")
	assert.Contains(t, got, "\nTop tags overall: sci-fi (1)\n")
	assert.NotContains(t, got, "Top tags, last 90 days")
	assert.Contains(t, got, "\nTop creators overall: Frank Herbert (1)\n")
}

func TestDigest_CapsEachList(t *testing.T) {
	var events []consumption.Event
	for i := range 40 {
		events = append(events, consumption.Event{
			Domain: consumption.DomainFeeds, ItemID: string(rune('a' + i)),
			Title: "Item", Creator: "", Tags: nil, At: time.Time{},
			Signal: consumption.Positive, Reason: "read in full",
		})
	}
	got := services.Digest(events, time.Now())
	assert.Equal(t, 25, strings.Count(got, "- Item (feeds"))
}
