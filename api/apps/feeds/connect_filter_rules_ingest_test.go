package feeds_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
)

func TestRefreshFeed_Scrape_CategoryRule_FiltersWithoutFetchingContent(t *testing.T) {
	client := newFeedsClient(t)
	base := uniqueBlogBase()
	indexURL := base + "/blog"
	oldURL := base + "/posts/older-announcement-with-long-title"
	newURL := base + "/posts/newer-announcement-with-long-title"
	cardHTML := func(u, title, cat string) string {
		return `<li><article><h2><a href="` + u + `">` + title + `</a></h2>` +
			`<div fs-list-field="category">` + cat + `</div></article></li>`
	}
	mockWebFetch.SetHTML(indexURL, `<!DOCTYPE html><html><body><ul>`+
		cardHTML(oldURL, "Older announcement with long title", "Product announcements")+
		`</ul></body></html>`)
	mockWebFetch.SetHTML(oldURL, articlePageHTML("Older"))
	mockWebFetch.SetHTML(newURL, articlePageHTML("Newer"))

	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{
			Url: indexURL, Kind: feedsv1.FeedKind_FEED_KIND_SCRAPE,
		}),
	)
	require.NoError(t, err)
	feedID := created.Msg.Feed.Id
	waitForItemCount(t, client, feedID, 1)

	rule := createRule(t, client, feedID,
		feedsv1.FilterRuleKind_FILTER_RULE_KIND_CATEGORY, "Product announcements")
	assert.Equal(t, int32(1), rule.FilteredCount)
	assert.Empty(t, inboxBySourceURL(t, client, feedID))

	mockWebFetch.SetHTML(indexURL, `<!DOCTYPE html><html><body><ul>`+
		cardHTML(newURL, "Newer announcement with long title", "Product Announcements")+
		cardHTML(oldURL, "Older announcement with long title", "Product announcements")+
		`</ul></body></html>`)
	refreshed, err := client.RefreshFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.RefreshFeedRequest{FeedId: feedID}),
	)
	require.NoError(t, err)

	assert.Equal(t, int32(0), refreshed.Msg.Ingested)
	assert.Zero(t, countFetches(newURL), "a filtered link must not be fetched")
	assert.Equal(t, 2, countSeenRows(t, feedID), "the filtered link is stored as seen")
	assert.Empty(t, inboxBySourceURL(t, client, feedID))
	assert.Equal(t, int32(2), listedRule(t, client, rule.Id).FilteredCount)
}

func TestRefreshFeed_RSS_CategoryRule_KeepsOnlyEmbeddedContent(t *testing.T) {
	client := newFeedsClient(t)
	base := uniqueBlogBase()
	feedURL := base + "/feed.xml"
	first := base + "/first"
	feedID := createRuleFeed(t, client, feedURL,
		ruleRSSItem{"First", first, []string{"Keep"}, itemContent},
	)
	rule := createRule(t, client, feedID,
		feedsv1.FilterRuleKind_FILTER_RULE_KIND_CATEGORY, "Skip")

	linked, embedded := base+"/linked", base+"/embedded"
	mockWebFetch.SetHTML(linked, articlePageHTML("Linked"))
	mockWebFetch.SetBody(feedURL, "application/rss+xml", []byte(ruleRSSXML(
		ruleRSSItem{"Linked", linked, []string{"skip"}, ""},
		ruleRSSItem{"Embedded", embedded, []string{"Skip"}, itemContent},
		ruleRSSItem{"First", first, []string{"Keep"}, itemContent},
	)))
	refreshed, err := client.RefreshFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.RefreshFeedRequest{FeedId: feedID}),
	)
	require.NoError(t, err)

	assert.Equal(t, int32(0), refreshed.Msg.Ingested)
	assert.Zero(t, countFetches(linked), "a filtered link must not be fetched")
	assert.Equal(t, int32(2), listedRule(t, client, rule.Id).FilteredCount)
	assert.Equal(t, map[string]string{linked: "", embedded: itemContent},
		storedContent(t, feedID, linked, embedded))
}

// storedContent maps each source URL to its stored body; filtered items
// have no RPC yet.
func storedContent(t *testing.T, feedID string, urls ...string) map[string]string {
	t.Helper()
	rows, err := testDB.Query(context.Background(), `
		SELECT source_url, content_html FROM feeds.items
		WHERE feed_id = $1 AND source_url = ANY($2) AND filtered_at IS NOT NULL
	`, feedID, urls)
	require.NoError(t, err)
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var u, body string
		require.NoError(t, rows.Scan(&u, &body))
		out[u] = body
	}
	require.NoError(t, rows.Err())
	return out
}

func TestCreateFilterRule_GlobalTitle_FiltersEveryFeedIncludingEmail(t *testing.T) {
	mux := getRoutes()
	emailFeedID, token, client := createEmailFeedFor(t, mux)
	stub := newResendReceivingStub(t)
	stub.html = "<p>Sponsored newsletter body</p>"
	marker := "promo-" + uuid.NewString()

	baseA, baseB := uniqueBlogBase(), uniqueBlogBase()
	feedA := createRuleFeed(t, client, baseA+"/feed.xml",
		ruleRSSItem{"Big " + marker + " sale", baseA + "/sale", nil, itemContent},
		ruleRSSItem{"Regular post", baseA + "/regular", nil, itemContent},
	)
	feedB := createRuleFeed(t, client, baseB+"/feed.xml",
		ruleRSSItem{"Another " + marker, baseB + "/another", nil, itemContent},
	)

	rule := createRule(t, client, "",
		feedsv1.FilterRuleKind_FILTER_RULE_KIND_TITLE, marker)
	assert.Empty(t, rule.FeedId)
	assert.Equal(t, int32(2), rule.FilteredCount)
	assert.Equal(t, []string{baseA + "/regular"}, keys(inboxBySourceURL(t, client, feedA)))
	assert.Empty(t, inboxBySourceURL(t, client, feedB))

	body := inboundPayload(token+"@mail.example.com", "This week: "+marker, "email-rule")
	headers := signEmailWebhookBody(t, emailWebhookSecret(), "msg-rule", body)
	postWebhook(mux, body, headers)

	sourceURL := "mailto:" + emailFeedID + "/" + messageIDFor("email-rule")
	assert.Nil(t, itemBySourceURL(t, client, sourceURL))
	assert.Equal(t, map[string]string{sourceURL: stub.html},
		storedContent(t, emailFeedID, sourceURL), "email body is kept")
	assert.Equal(t, int32(3), listedRule(t, client, rule.Id).FilteredCount)
}

func keys(m map[string]*feedsv1.Item) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
