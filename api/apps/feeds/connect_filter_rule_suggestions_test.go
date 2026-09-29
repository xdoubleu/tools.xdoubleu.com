package feeds_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
	"tools.xdoubleu.com/gen/feeds/v1/feedsv1connect"
)

const suggestedCategory = "Product Announcements"

// createCategoryFeed subscribes to a feed of n suggestedCategory items and
// returns its id with the items' ids, in feed order.
func createCategoryFeed(
	t *testing.T,
	client feedsv1connect.FeedServiceClient,
	n int,
) (string, []string) {
	t.Helper()
	return createCategoryFeedOf(t, client, n, suggestedCategory)
}

func createCategoryFeedOf(
	t *testing.T,
	client feedsv1connect.FeedServiceClient,
	n int,
	category string,
) (string, []string) {
	t.Helper()
	base := uniqueBlogBase()
	items := make([]ruleRSSItem, n)
	for i := range items {
		items[i] = ruleRSSItem{
			fmt.Sprintf("Post %d", i), fmt.Sprintf("%s/post-%d", base, i),
			[]string{category}, itemContent,
		}
	}
	feedID := createRuleFeed(t, client, base+"/feed.xml", items...)
	inbox := inboxBySourceURL(t, client, feedID)
	ids := make([]string, n)
	for i, it := range items {
		ids[i] = inbox[it.link].Id
	}
	return feedID, ids
}

func setItems(
	t *testing.T,
	client feedsv1connect.FeedServiceClient,
	ids []string,
	build func(id string) *feedsv1.UpdateItemRequest,
) {
	t.Helper()
	for _, id := range ids {
		updateItem(t, client, build(id))
	}
}

func markRead(t *testing.T, client feedsv1connect.FeedServiceClient, ids []string) {
	t.Helper()
	yes := true
	setItems(t, client, ids, func(id string) *feedsv1.UpdateItemRequest {
		return &feedsv1.UpdateItemRequest{ItemId: id, Read: &yes}
	})
}

func bookmark(t *testing.T, client feedsv1connect.FeedServiceClient, ids []string) {
	t.Helper()
	yes := true
	setItems(t, client, ids, func(id string) *feedsv1.UpdateItemRequest {
		return &feedsv1.UpdateItemRequest{ItemId: id, Bookmarked: &yes}
	})
}

// suggestionFor returns feedID's suggestion, or nil.
func suggestionFor(
	t *testing.T,
	client feedsv1connect.FeedServiceClient,
	feedID string,
) *feedsv1.FilterRuleSuggestion {
	t.Helper()
	resp, err := client.GetFilterRuleSuggestions(
		context.Background(),
		connect.NewRequest(&feedsv1.GetFilterRuleSuggestionsRequest{}),
	)
	require.NoError(t, err)
	var out *feedsv1.FilterRuleSuggestion
	for _, s := range resp.Msg.Suggestions {
		if s.FeedId == feedID {
			require.Nil(t, out, "feed %s has more than one suggestion", feedID)
			out = s
		}
	}
	return out
}

func dismissSuggestion(
	client feedsv1connect.FeedServiceClient,
	feedID, category string,
) error {
	_, err := client.DismissFilterRuleSuggestion(
		context.Background(),
		connect.NewRequest(&feedsv1.DismissFilterRuleSuggestionRequest{
			FeedId: feedID, Category: category,
		}),
	)
	return err
}

func TestFilterRuleSuggestions_TenItemsTenPercentRead(t *testing.T) {
	client := newFeedsClient(t)
	feedID, ids := createCategoryFeed(t, client, 10)
	markRead(t, client, ids[:1])

	s := suggestionFor(t, client, feedID)

	require.NotNil(t, s)
	assert.Equal(t, suggestedCategory, s.Category)
	assert.Equal(t, "Rules Blog", s.FeedTitle)
	assert.NotEmpty(t, s.FeedUrl)
	assert.Equal(t, int32(10), s.ItemCount)
	assert.Equal(t, int32(1), s.ReadCount)
	assert.InDelta(t, 0.1, s.ReadRate, 1e-9)
}

func TestFilterRuleSuggestions_NineItems_None(t *testing.T) {
	client := newFeedsClient(t)
	feedID, _ := createCategoryFeed(t, client, 9)

	assert.Nil(t, suggestionFor(t, client, feedID))
}

func TestFilterRuleSuggestions_ElevenPercentRead_None(t *testing.T) {
	client := newFeedsClient(t)
	feedID, ids := createCategoryFeed(t, client, 18)
	markRead(t, client, ids[:2]) // 2 of 18 is 11%

	assert.Nil(t, suggestionFor(t, client, feedID))
}

func TestFilterRuleSuggestions_OnlyCountsLast90Days(t *testing.T) {
	client := newFeedsClient(t)
	feedID, ids := createCategoryFeed(t, client, 10)
	_, err := testDB.Exec(context.Background(), `
		UPDATE feeds.items SET published_at = now() - interval '91 days'
		WHERE id = $1
	`, ids[0])
	require.NoError(t, err)

	assert.Nil(t, suggestionFor(t, client, feedID))
}

// A rule can't hold a category this long, so its suggestion couldn't be
// acted on.
func TestFilterRuleSuggestions_SkipCategoriesTooLongForARule(t *testing.T) {
	client := newFeedsClient(t)
	feedID, _ := createCategoryFeedOf(t, client, 10, strings.Repeat("x", 201))

	assert.Nil(t, suggestionFor(t, client, feedID))
}

func TestFilterRuleSuggestions_HiddenOnceARuleCoversTheCategory(t *testing.T) {
	cases := []struct {
		name     string
		ruleFeed func(feedID string) string
	}{
		{"per-feed rule", func(feedID string) string { return feedID }},
		{"global rule", func(string) string { return "" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := newFeedsClient(t)
			feedID, ids := createCategoryFeed(t, client, 10)
			// Bookmarked items stay unfiltered, so only the rule can hide it.
			bookmark(t, client, ids)
			otherFeedID, _ := createCategoryFeed(t, client, 1)
			createRule(t, client, otherFeedID,
				feedsv1.FilterRuleKind_FILTER_RULE_KIND_CATEGORY, suggestedCategory)
			require.NotNil(t, suggestionFor(t, client, feedID))

			createRule(t, client, c.ruleFeed(feedID),
				feedsv1.FilterRuleKind_FILTER_RULE_KIND_CATEGORY,
				"product ANNOUNCEMENTS")

			assert.Len(t, inboxBySourceURL(t, client, feedID), len(ids))
			assert.Nil(t, suggestionFor(t, client, feedID))
		})
	}
}

func TestFilterRuleSuggestions_TitleRuleDoesNotCoverTheCategory(t *testing.T) {
	client := newFeedsClient(t)
	feedID, _ := createCategoryFeed(t, client, 10)
	createRule(t, client, feedID,
		feedsv1.FilterRuleKind_FILTER_RULE_KIND_TITLE, suggestedCategory)

	assert.NotNil(t, suggestionFor(t, client, feedID))
}

func TestDismissFilterRuleSuggestion_NeverReturns(t *testing.T) {
	client := newFeedsClient(t)
	feedID, _ := createCategoryFeed(t, client, 10)
	require.NotNil(t, suggestionFor(t, client, feedID))

	require.NoError(t, dismissSuggestion(client, feedID, " product  announcements "))
	assert.Nil(t, suggestionFor(t, client, feedID))

	require.NoError(t, dismissSuggestion(client, feedID, suggestedCategory))
	assert.Nil(t, suggestionFor(t, client, feedID))
}

func TestDismissFilterRuleSuggestion_Errors(t *testing.T) {
	client := newFeedsClient(t)
	feedID, _ := createCategoryFeed(t, client, 1)
	cases := []struct {
		name, feedID, category string
		code                   connect.Code
	}{
		{"blank category", feedID, "  ", connect.CodeInvalidArgument},
		{"invalid feed id", "nope", "x", connect.CodeInvalidArgument},
		{"unknown feed", uuid.NewString(), "x", connect.CodeNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := dismissSuggestion(client, c.feedID, c.category)
			assert.Equal(t, c.code, connect.CodeOf(err))
		})
	}
}

func TestFilterRuleSuggestion_CreatingItsRuleFiltersUnreadItems(t *testing.T) {
	client := newFeedsClient(t)
	feedID, ids := createCategoryFeed(t, client, 12)
	markRead(t, client, ids[:1])
	s := suggestionFor(t, client, feedID)
	require.NotNil(t, s)

	rule := createRule(t, client, s.FeedId,
		feedsv1.FilterRuleKind_FILTER_RULE_KIND_CATEGORY, s.Category)

	assert.Equal(t, int32(11), rule.FilteredCount)
	inbox := inboxBySourceURL(t, client, feedID)
	assert.Len(t, inbox, 1)
	assert.Nil(t, suggestionFor(t, client, feedID))
}

func TestFilterRuleSuggestions_ExcludeFilteredAndRestoredItems(t *testing.T) {
	client := newFeedsClient(t)
	feedID, _ := createCategoryFeed(t, client, 10)
	require.NotNil(t, suggestionFor(t, client, feedID))

	// "Post 0" only: the other titles don't contain it.
	createRule(t, client, feedID,
		feedsv1.FilterRuleKind_FILTER_RULE_KIND_TITLE, "Post 0")
	assert.Nil(t, suggestionFor(t, client, feedID), "filtered item counted")

	var filteredID string
	for _, item := range filteredBySourceURL(t, client, &feedID) {
		filteredID = item.Id
	}
	require.NotEmpty(t, filteredID)
	restoreItem(t, client, filteredID)
	assert.Nil(t, suggestionFor(t, client, feedID), "restored item counted")
}
