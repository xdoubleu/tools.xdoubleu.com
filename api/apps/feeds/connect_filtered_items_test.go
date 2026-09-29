package feeds_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
	"tools.xdoubleu.com/gen/feeds/v1/feedsv1connect"
)

// filteredBySourceURL lists the caller's filtered items, optionally for one
// feed.
func filteredBySourceURL(
	t *testing.T,
	client feedsv1connect.FeedServiceClient,
	feedID *string,
) map[string]*feedsv1.Item {
	t.Helper()
	resp, err := client.ListFeedItems(
		context.Background(),
		connect.NewRequest(&feedsv1.ListFeedItemsRequest{
			FeedId: feedID, FilteredOnly: proto.Bool(true), Limit: 100,
		}),
	)
	require.NoError(t, err)
	out := map[string]*feedsv1.Item{}
	for _, item := range resp.Msg.Items {
		out[item.SourceUrl] = item
	}
	return out
}

func restoreItem(
	t *testing.T,
	client feedsv1connect.FeedServiceClient,
	itemID string,
) *feedsv1.Item {
	t.Helper()
	resp, err := client.RestoreFeedItem(
		context.Background(),
		connect.NewRequest(&feedsv1.RestoreFeedItemRequest{ItemId: itemID}),
	)
	require.NoError(t, err)
	return resp.Msg.Item
}

func unreadInbox(
	t *testing.T,
	client feedsv1connect.FeedServiceClient,
	feedID string,
) map[string]*feedsv1.Item {
	t.Helper()
	resp, err := client.ListFeedItems(
		context.Background(),
		connect.NewRequest(&feedsv1.ListFeedItemsRequest{
			FeedId: &feedID, UnreadOnly: proto.Bool(true),
		}),
	)
	require.NoError(t, err)
	out := map[string]*feedsv1.Item{}
	for _, item := range resp.Msg.Items {
		out[item.SourceUrl] = item
	}
	return out
}

// ingestFilteredRSSItem subscribes to an RSS feed, adds a category rule, then
// polls in an item with no embedded content that the rule filters at ingest.
// Returns the feed id, the item's source URL and the rule.
func ingestFilteredRSSItem(
	t *testing.T,
	client feedsv1connect.FeedServiceClient,
) (string, string, *feedsv1.FilterRule) {
	t.Helper()
	base := uniqueBlogBase()
	feedURL := base + "/feed.xml"
	first := base + "/first"
	feedID := createRuleFeed(t, client, feedURL,
		ruleRSSItem{"First", first, []string{"Keep"}, itemContent},
	)
	rule := createRule(t, client, feedID,
		feedsv1.FilterRuleKind_FILTER_RULE_KIND_CATEGORY, "Skip")

	linked := base + "/linked"
	mockWebFetch.SetHTML(linked, articlePageHTML("Linked"))
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(ruleRSSXML(
		ruleRSSItem{"Linked sponsored post", linked, []string{"Skip"}, ""},
		ruleRSSItem{"First", first, []string{"Keep"}, itemContent},
	)))
	_, err := client.RefreshFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.RefreshFeedRequest{FeedId: feedID}),
	)
	require.NoError(t, err)
	require.Zero(t, countFetches(linked))
	return feedID, linked, rule
}

func TestListFeedItems_FilteredOnly_ShowsRuleAndHidesInbox(t *testing.T) {
	client := newFeedsClient(t)
	feedID, linked, rule := ingestFilteredRSSItem(t, client)

	filtered := filteredBySourceURL(t, client, &feedID)
	require.Len(t, filtered, 1)
	item := filtered[linked]
	require.NotNil(t, item)
	assert.NotEmpty(t, item.FilteredAt)
	assert.Empty(t, item.RestoredAt)
	assert.False(t, item.HasContent)
	assert.Equal(t, []string{"Skip"}, item.Categories)
	require.NotNil(t, item.FilterRule)
	assert.Equal(t, rule.Id, item.FilterRule.Id)
	assert.Equal(
		t,
		feedsv1.FilterRuleKind_FILTER_RULE_KIND_CATEGORY,
		item.FilterRule.Kind,
	)
	assert.Equal(t, "Skip", item.FilterRule.Value)
	assert.Equal(t, feedID, item.FilterRule.FeedId)

	assert.NotContains(t, inboxBySourceURL(t, client, feedID), linked)
}

func TestRestoreFeedItem_RSS_FetchesContentAndReturnsUnreadToInbox(t *testing.T) {
	client := newFeedsClient(t)
	feedID, linked, rule := ingestFilteredRSSItem(t, client)
	filtered := filteredBySourceURL(t, client, &feedID)[linked]
	require.NotNil(t, filtered)
	statsBefore := feedItemCount(t, client, feedID)

	restored := restoreItem(t, client, filtered.Id)

	assert.Equal(t, 1, countFetches(linked), "restore fetches the missing content")
	assert.True(t, restored.HasContent)
	assert.Empty(t, restored.FilteredAt)
	assert.Nil(t, restored.FilterRule)
	assert.NotEmpty(t, restored.RestoredAt)
	assert.Empty(t, restored.ReadAt)
	assert.Empty(t, restored.IngestError)
	assert.Equal(t, filtered.PublishedAt, restored.PublishedAt)

	inbox := unreadInbox(t, client, feedID)
	require.Contains(t, inbox, linked)
	assert.Equal(t, filtered.PublishedAt, inbox[linked].PublishedAt)
	assert.Equal(t, statsBefore+1, feedItemCount(t, client, feedID))
	assert.NotContains(t, filteredBySourceURL(t, client, &feedID), linked)
	assert.Equal(t, int32(0), listedRule(t, client, rule.Id).FilteredCount)

	body, err := client.GetFeedItem(
		context.Background(),
		connect.NewRequest(&feedsv1.GetFeedItemRequest{ItemId: restored.Id}),
	)
	require.NoError(t, err)
	assert.Contains(t, body.Msg.Item.ContentHtml, "Lorem ipsum")
}

func TestRestoreFeedItem_Scrape_FetchesContent(t *testing.T) {
	client := newFeedsClient(t)
	base := uniqueBlogBase()
	indexURL := base + "/blog"
	postURL := base + "/posts/product-launch-with-long-title"
	mockWebFetch.SetHTML(indexURL, `<!DOCTYPE html><html><body><ul><li><article>`+
		`<h2><a href="`+postURL+`">Product launch with long title</a></h2>`+
		`<div fs-list-field="category">Launches</div></article></li></ul></body></html>`)
	mockWebFetch.SetHTML(postURL, articlePageHTML("Launch"))
	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{
			Url: indexURL, Kind: feedsv1.FeedKind_FEED_KIND_SCRAPE,
		}),
	)
	require.NoError(t, err)
	feedID := created.Msg.Feed.Id
	waitForItemCount(t, client, feedID, 1)
	createRule(t, client, feedID,
		feedsv1.FilterRuleKind_FILTER_RULE_KIND_CATEGORY, "Launches")

	// Filtered retroactively, so blank the body to mimic an ingest-time filter.
	_, err = testDB.Exec(context.Background(),
		`UPDATE feeds.items SET content_html = '' WHERE feed_id = $1`, feedID)
	require.NoError(t, err)
	filtered := filteredBySourceURL(t, client, &feedID)[postURL]
	require.NotNil(t, filtered)
	require.False(t, filtered.HasContent)
	fetchesBefore := countFetches(postURL)

	restored := restoreItem(t, client, filtered.Id)

	assert.Equal(t, fetchesBefore+1, countFetches(postURL))
	assert.True(t, restored.HasContent)
	assert.Contains(t, unreadInbox(t, client, feedID), postURL)
}

func TestRestoreFeedItem_FetchFailure_StillRestores(t *testing.T) {
	client := newFeedsClient(t)
	feedID, linked, _ := ingestFilteredRSSItem(t, client)
	delete(mockWebFetch.Responses, linked)
	filtered := filteredBySourceURL(t, client, &feedID)[linked]
	require.NotNil(t, filtered)

	restored := restoreItem(t, client, filtered.Id)

	assert.False(t, restored.HasContent)
	assert.Empty(t, restored.FilteredAt)
	assert.Contains(t, unreadInbox(t, client, feedID), linked)
}

func TestRestoreFeedItem_KeepsExistingContentWithoutFetching(t *testing.T) {
	client := newFeedsClient(t)
	base := uniqueBlogBase()
	post := base + "/embedded"
	feedID := createRuleFeed(t, client, base+"/feed.xml",
		ruleRSSItem{"Embedded", post, []string{"Noise"}, itemContent},
	)
	createRule(t, client, feedID,
		feedsv1.FilterRuleKind_FILTER_RULE_KIND_CATEGORY, "Noise")
	filtered := filteredBySourceURL(t, client, &feedID)[post]
	require.NotNil(t, filtered)
	require.True(t, filtered.HasContent)

	restored := restoreItem(t, client, filtered.Id)

	assert.True(t, restored.HasContent)
	assert.Zero(t, countFetches(post))
}

func TestCreateFilterRule_NeverRefiltersRestoredItem(t *testing.T) {
	client := newFeedsClient(t)
	feedID, linked, _ := ingestFilteredRSSItem(t, client)
	filtered := filteredBySourceURL(t, client, &feedID)[linked]
	require.NotNil(t, filtered)
	restoreItem(t, client, filtered.Id)

	rule := createRule(t, client, feedID,
		feedsv1.FilterRuleKind_FILTER_RULE_KIND_TITLE, "sponsored")

	assert.Equal(t, int32(0), rule.FilteredCount)
	assert.Contains(t, unreadInbox(t, client, feedID), linked)
}

func TestRestoreFeedItem_DeletedRule_ShowsNoRuleAndRestores(t *testing.T) {
	client := newFeedsClient(t)
	feedID, linked, rule := ingestFilteredRSSItem(t, client)
	_, err := client.DeleteFilterRule(
		context.Background(),
		connect.NewRequest(&feedsv1.DeleteFilterRuleRequest{RuleId: rule.Id}),
	)
	require.NoError(t, err)

	filtered := filteredBySourceURL(t, client, &feedID)[linked]
	require.NotNil(t, filtered)
	assert.NotEmpty(t, filtered.FilteredAt)
	assert.Nil(t, filtered.FilterRule)

	restoreItem(t, client, filtered.Id)
	assert.Contains(t, unreadInbox(t, client, feedID), linked)
}

// seedFilteredItem stores a filtered item on a new feed owned by owner and
// returns the feed and item ids.
func seedFilteredItem(t *testing.T, owner string) (string, string) {
	t.Helper()
	var feedID, itemID string
	ctx := context.Background()
	base := uniqueBlogBase()
	require.NoError(t, testDB.QueryRow(ctx, `
		INSERT INTO feeds.feeds (user_id, url, title, source_type)
		VALUES ($1, $2, 'Foreign', 'rss') RETURNING id
	`, owner, base+"/feed.xml").Scan(&feedID))
	require.NoError(t, testDB.QueryRow(ctx, `
		INSERT INTO feeds.items
		    (feed_id, guid, title, source_url, published_at, filtered_at)
		VALUES ($1, $2, 'Foreign item', $2, now(), now()) RETURNING id
	`, feedID, base+"/post").Scan(&itemID))
	return feedID, itemID
}

func TestFilteredItems_OtherUserCannotListOrRestore(t *testing.T) {
	client := newFeedsClient(t)
	foreignFeedID, foreignItemID := seedFilteredItem(
		t,
		"another-user-"+uuid.NewString(),
	)

	for _, item := range filteredBySourceURL(t, client, nil) {
		assert.NotEqual(t, foreignItemID, item.Id)
	}
	assert.Empty(t, filteredBySourceURL(t, client, &foreignFeedID))

	_, err := client.RestoreFeedItem(
		context.Background(),
		connect.NewRequest(&feedsv1.RestoreFeedItemRequest{ItemId: foreignItemID}),
	)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	var stillFiltered bool
	require.NoError(t, testDB.QueryRow(context.Background(),
		`SELECT filtered_at IS NOT NULL FROM feeds.items WHERE id = $1`,
		foreignItemID,
	).Scan(&stillFiltered))
	assert.True(t, stillFiltered)
}

func TestRestoreFeedItem_Errors(t *testing.T) {
	client := newFeedsClient(t)
	base := uniqueBlogBase()
	post := base + "/visible"
	feedID := createRuleFeed(t, client, base+"/feed.xml",
		ruleRSSItem{"Visible", post, nil, itemContent},
	)
	visible := inboxBySourceURL(t, client, feedID)[post]
	require.NotNil(t, visible)

	cases := []struct {
		name, itemID string
		code         connect.Code
	}{
		{"invalid id", "nope", connect.CodeInvalidArgument},
		{"unknown item", uuid.NewString(), connect.CodeNotFound},
		{"not filtered", visible.Id, connect.CodeFailedPrecondition},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := client.RestoreFeedItem(
				context.Background(),
				connect.NewRequest(&feedsv1.RestoreFeedItemRequest{ItemId: c.itemID}),
			)
			assert.Equal(t, c.code, connect.CodeOf(err))
		})
	}
}
