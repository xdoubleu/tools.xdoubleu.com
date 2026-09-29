package feeds_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
	"tools.xdoubleu.com/gen/feeds/v1/feedsv1connect"
)

// ruleRSSItem is an RSS <item>; empty content makes ingest fetch the link.
type ruleRSSItem struct {
	title, link string
	categories  []string
	content     string
}

func ruleRSSXML(items ...ruleRSSItem) string {
	body := `<?xml version="1.0"?><rss version="2.0" ` +
		`xmlns:content="http://purl.org/rss/1.0/modules/content/">` +
		`<channel><title>Rules Blog</title>`
	for _, it := range items {
		body += `<item><title>` + it.title + `</title><link>` + it.link +
			`</link><guid>` + it.link + `</guid>`
		for _, c := range it.categories {
			body += `<category>` + c + `</category>`
		}
		if it.content != "" {
			body += `<content:encoded><![CDATA[` + it.content + `]]></content:encoded>`
		}
		body += `</item>`
	}
	return body + `</channel></rss>`
}

// createRuleFeed subscribes to an RSS feed at feedURL serving items and
// waits for all of them to import.
func createRuleFeed(
	t *testing.T,
	client feedsv1connect.FeedServiceClient,
	feedURL string,
	items ...ruleRSSItem,
) string {
	t.Helper()
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(ruleRSSXML(items...)))
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{Url: feedURL}),
	)
	require.NoError(t, err)
	waitForItemCount(t, client, created.Msg.Feed.Id, len(items))
	return created.Msg.Feed.Id
}

// inboxBySourceURL lists feedID's inbox items, read ones included.
func inboxBySourceURL(
	t *testing.T,
	client feedsv1connect.FeedServiceClient,
	feedID string,
) map[string]*feedsv1.Item {
	t.Helper()
	resp, err := client.ListFeedItems(
		context.Background(),
		connect.NewRequest(&feedsv1.ListFeedItemsRequest{FeedId: &feedID}),
	)
	require.NoError(t, err)
	out := map[string]*feedsv1.Item{}
	for _, item := range resp.Msg.Items {
		out[item.SourceUrl] = item
	}
	return out
}

// createRule creates a filter rule and deletes it when the test ends, so a
// global rule can't leak into later tests.
func createRule(
	t *testing.T,
	client feedsv1connect.FeedServiceClient,
	feedID string,
	kind feedsv1.FilterRuleKind,
	value string,
) *feedsv1.FilterRule {
	t.Helper()
	resp, err := client.CreateFilterRule(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFilterRuleRequest{
			FeedId: feedID, Kind: kind, Value: value,
		}),
	)
	require.NoError(t, err)
	rule := resp.Msg.Rule
	t.Cleanup(func() {
		_, _ = client.DeleteFilterRule(
			context.Background(),
			connect.NewRequest(&feedsv1.DeleteFilterRuleRequest{RuleId: rule.Id}),
		)
	})
	return rule
}

func listedRule(
	t *testing.T,
	client feedsv1connect.FeedServiceClient,
	ruleID string,
) *feedsv1.FilterRule {
	t.Helper()
	resp, err := client.ListFilterRules(
		context.Background(),
		connect.NewRequest(&feedsv1.ListFilterRulesRequest{}),
	)
	require.NoError(t, err)
	for _, r := range resp.Msg.Rules {
		if r.Id == ruleID {
			return r
		}
	}
	return nil
}

func feedItemCount(
	t *testing.T,
	client feedsv1connect.FeedServiceClient,
	feedID string,
) int32 {
	t.Helper()
	resp, err := client.GetFeedStats(
		context.Background(), connect.NewRequest(&feedsv1.GetFeedStatsRequest{}),
	)
	require.NoError(t, err)
	for _, s := range resp.Msg.Stats {
		if s.FeedId == feedID {
			return s.ItemCount
		}
	}
	t.Fatalf("feed %s has no stats", feedID)
	return 0
}

func updateItem(
	t *testing.T,
	client feedsv1connect.FeedServiceClient,
	req *feedsv1.UpdateItemRequest,
) {
	t.Helper()
	_, err := client.UpdateItem(context.Background(), connect.NewRequest(req))
	require.NoError(t, err)
}

func TestCreateFilterRule_Category_FiltersOnlyUnreadUntouchedItems(t *testing.T) {
	client := newFeedsClient(t)
	base := uniqueBlogBase()
	const cat = "Product announcements"
	unread, read := base+"/unread", base+"/read"
	bookmarked, dismissed := base+"/bookmarked", base+"/dismissed"
	other := base + "/other"
	feedID := createRuleFeed(t, client, base+"/feed.xml",
		ruleRSSItem{"Unread", unread, []string{cat}, itemContent},
		ruleRSSItem{"Read", read, []string{cat}, itemContent},
		ruleRSSItem{"Bookmarked", bookmarked, []string{cat}, itemContent},
		ruleRSSItem{"Dismissed", dismissed, []string{cat}, itemContent},
		ruleRSSItem{"Other", other, []string{"Research"}, itemContent},
	)
	items := inboxBySourceURL(t, client, feedID)
	yes := true
	updateItem(
		t,
		client,
		&feedsv1.UpdateItemRequest{ItemId: items[read].Id, Read: &yes},
	)
	updateItem(t, client, &feedsv1.UpdateItemRequest{
		ItemId: items[bookmarked].Id, Bookmarked: &yes,
	})
	updateItem(t, client, &feedsv1.UpdateItemRequest{
		ItemId: items[dismissed].Id, Dismissed: &yes,
	})
	require.Equal(t, int32(5), feedItemCount(t, client, feedID))

	rule := createRule(t, client, feedID,
		feedsv1.FilterRuleKind_FILTER_RULE_KIND_CATEGORY, "product ANNOUNCEMENTS")

	assert.Equal(t, int32(1), rule.FilteredCount)
	assert.Equal(t, feedID, rule.FeedId)
	inbox := inboxBySourceURL(t, client, feedID)
	assert.NotContains(t, inbox, unread)
	assert.Contains(t, inbox, read)
	assert.Contains(t, inbox, bookmarked)
	assert.Contains(t, inbox, other)
	assert.Equal(t, int32(4), feedItemCount(t, client, feedID))

	listed := listedRule(t, client, rule.Id)
	require.NotNil(t, listed)
	assert.Equal(t, int32(1), listed.FilteredCount)
	assert.Equal(t, feedsv1.FilterRuleKind_FILTER_RULE_KIND_CATEGORY, listed.Kind)
}

func TestDeleteFilterRule_ItemsStayFiltered(t *testing.T) {
	client := newFeedsClient(t)
	base := uniqueBlogBase()
	hidden := base + "/hidden"
	feedID := createRuleFeed(t, client, base+"/feed.xml",
		ruleRSSItem{"Hidden", hidden, []string{"Noise"}, itemContent},
	)
	rule := createRule(t, client, feedID,
		feedsv1.FilterRuleKind_FILTER_RULE_KIND_CATEGORY, "Noise")
	require.Equal(t, int32(1), rule.FilteredCount)

	_, err := client.DeleteFilterRule(
		context.Background(),
		connect.NewRequest(&feedsv1.DeleteFilterRuleRequest{RuleId: rule.Id}),
	)
	require.NoError(t, err)

	assert.Nil(t, listedRule(t, client, rule.Id))
	assert.Empty(t, inboxBySourceURL(t, client, feedID))
	var filteredAtSet, ruleIDNull bool
	require.NoError(t, testDB.QueryRow(context.Background(), `
		SELECT filtered_at IS NOT NULL, filtered_rule_id IS NULL
		FROM feeds.items WHERE feed_id = $1 AND source_url = $2
	`, feedID, hidden).Scan(&filteredAtSet, &ruleIDNull))
	assert.True(t, filteredAtSet)
	assert.True(t, ruleIDNull)

	_, err = client.DeleteFilterRule(
		context.Background(),
		connect.NewRequest(&feedsv1.DeleteFilterRuleRequest{RuleId: rule.Id}),
	)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestFilteredItems_ExcludedFromOpenItemCounts(t *testing.T) {
	client := newFeedsClient(t)
	base := uniqueBlogBase()
	feedURL := base + "/feed.xml"
	feedID := createRuleFeed(t, client, feedURL,
		ruleRSSItem{"Hidden", base + "/hidden", []string{"Noise"}, itemContent},
		ruleRSSItem{"Shown", base + "/shown", nil, itemContent},
	)
	openCount := func() int {
		open, err := testApp.ListOpenItems(context.Background())
		require.NoError(t, err)
		for _, o := range open {
			if o.URL == feedURL {
				return o.Count
			}
		}
		return 0
	}
	require.Equal(t, 2, openCount())

	createRule(t, client, feedID,
		feedsv1.FilterRuleKind_FILTER_RULE_KIND_CATEGORY, "Noise")

	assert.Equal(t, 1, openCount())
}

func TestCreateFilterRule_Errors(t *testing.T) {
	client := newFeedsClient(t)
	base := uniqueBlogBase()
	feedID := createRuleFeed(t, client, base+"/feed.xml",
		ruleRSSItem{"Post", base + "/post", nil, itemContent},
	)
	createRule(t, client, feedID, feedsv1.FilterRuleKind_FILTER_RULE_KIND_TITLE, "Dup")

	cases := []struct {
		name string
		req  *feedsv1.CreateFilterRuleRequest
		code connect.Code
	}{
		{"unspecified kind", &feedsv1.CreateFilterRuleRequest{
			FeedId: feedID, Value: "x",
		}, connect.CodeInvalidArgument},
		{"blank value", &feedsv1.CreateFilterRuleRequest{
			FeedId: feedID, Kind: feedsv1.FilterRuleKind_FILTER_RULE_KIND_TITLE,
			Value: "   ",
		}, connect.CodeInvalidArgument},
		{"invalid feed id", &feedsv1.CreateFilterRuleRequest{
			FeedId: "nope", Kind: feedsv1.FilterRuleKind_FILTER_RULE_KIND_TITLE,
			Value: "x",
		}, connect.CodeInvalidArgument},
		{"unknown feed", &feedsv1.CreateFilterRuleRequest{
			FeedId: uuid.NewString(), Kind: feedsv1.FilterRuleKind_FILTER_RULE_KIND_TITLE,
			Value: "x",
		}, connect.CodeNotFound},
		{"duplicate ignoring case", &feedsv1.CreateFilterRuleRequest{
			FeedId: feedID, Kind: feedsv1.FilterRuleKind_FILTER_RULE_KIND_TITLE,
			Value: " dup ",
		}, connect.CodeAlreadyExists},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := client.CreateFilterRule(
				context.Background(),
				connect.NewRequest(c.req),
			)
			var cerr *connect.Error
			require.True(t, errors.As(err, &cerr), "want a connect error, got %v", err)
			assert.Equal(t, c.code, cerr.Code())
		})
	}
}

func TestDeleteFilterRule_InvalidID(t *testing.T) {
	client := newFeedsClient(t)
	_, err := client.DeleteFilterRule(
		context.Background(),
		connect.NewRequest(&feedsv1.DeleteFilterRuleRequest{RuleId: "nope"}),
	)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
