package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/feeds/internal/models"
)

func TestMatchFilterRule(t *testing.T) {
	feedID := uuid.New()
	otherFeedID := uuid.New()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	rule := func(feed *uuid.UUID, kind, value string, age int) models.FilterRule {
		//nolint:exhaustruct // UserID/FilteredCount are irrelevant to matching
		return models.FilterRule{
			ID:        uuid.New(),
			FeedID:    feed,
			Kind:      kind,
			Value:     value,
			CreatedAt: t0.Add(-time.Duration(age) * time.Hour),
		}
	}
	announcements := rule(&feedID, models.FilterRuleKindCategory,
		"Product announcements", 1)
	otherFeed := rule(&otherFeedID, models.FilterRuleKindCategory,
		"Product announcements", 1)
	globalTitle := rule(nil, models.FilterRuleKindTitle, "sponsored", 1)
	olderTitle := rule(nil, models.FilterRuleKindTitle, "weekly", 5)
	newerCategory := rule(&feedID, models.FilterRuleKindCategory, "News", 2)

	cases := []struct {
		name       string
		rules      []models.FilterRule
		title      string
		categories []string
		want       *models.FilterRule
	}{
		{"no rules", nil, "Anything", []string{"News"}, nil},
		{
			"category matches ignoring case",
			[]models.FilterRule{announcements},
			"Launch", []string{"Research", "product ANNOUNCEMENTS"},
			&announcements,
		},
		{
			"category must match whole value",
			[]models.FilterRule{announcements},
			"Launch", []string{"Product announcements 2026"},
			nil,
		},
		{
			"category rule never matches the title",
			[]models.FilterRule{announcements},
			"Product announcements", nil,
			nil,
		},
		{
			"rule for another feed is ignored",
			[]models.FilterRule{otherFeed},
			"Launch", []string{"Product announcements"},
			nil,
		},
		{
			"global title substring matches ignoring case",
			[]models.FilterRule{globalTitle},
			"A SPONSORED post", nil,
			&globalTitle,
		},
		{
			"title rule never matches a category",
			[]models.FilterRule{globalTitle},
			"Plain post", []string{"sponsored"},
			nil,
		},
		{
			"oldest of several matches wins",
			[]models.FilterRule{newerCategory, olderTitle},
			"Weekly roundup", []string{"news"},
			&olderTitle,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := matchFilterRule(c.rules, feedID, c.title, c.categories)
			if c.want == nil {
				assert.Nil(t, got)
				return
			}
			if assert.NotNil(t, got) {
				assert.Equal(t, c.want.ID, got.ID)
			}
		})
	}
}
