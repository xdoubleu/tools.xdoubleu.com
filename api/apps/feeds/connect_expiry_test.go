package feeds_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
	"tools.xdoubleu.com/gen/feeds/v1/feedsv1connect"
)

func seedExpiryFeed(t *testing.T, expiryDays int) string {
	t.Helper()
	var feedID string
	require.NoError(t, testDB.QueryRow(context.Background(), `
		INSERT INTO feeds.feeds (user_id, url, title, source_type, expiry_days)
		VALUES ($1, $2, 'Expiry feed', 'rss', $3) RETURNING id
	`, userID, uniqueBlogBase()+"/feed.xml", expiryDays).Scan(&feedID))
	return feedID
}

// seedAgedItem inserts an item that arrived age ago; extra is appended to the
// column assignments, e.g. "read_at = now()".
func seedAgedItem(
	t *testing.T,
	feedID, title string,
	age time.Duration,
	extra string,
) string {
	t.Helper()
	var itemID string
	require.NoError(t, testDB.QueryRow(context.Background(), `
		INSERT INTO feeds.items
		    (feed_id, guid, title, source_url, published_at, created_at)
		VALUES ($1, $5, $2, $3, now(), now() - make_interval(secs => $4))
		RETURNING id
	`, feedID, title, "https://example.com/"+title,
		age.Seconds(), uuid.NewString()).Scan(&itemID))
	if extra != "" {
		_, err := testDB.Exec(context.Background(),
			`UPDATE feeds.items SET `+extra+` WHERE id = $1`, itemID)
		require.NoError(t, err)
	}
	return itemID
}

func titlesIn(
	t *testing.T,
	client feedsv1connect.FeedServiceClient,
	feedID string,
	req *feedsv1.ListFeedItemsRequest,
) map[string]*feedsv1.Item {
	t.Helper()
	req.FeedId = &feedID
	req.Limit = 100
	resp, err := client.ListFeedItems(context.Background(), req)
	require.NoError(t, err)
	out := map[string]*feedsv1.Item{}
	for _, item := range resp.Items {
		out[item.Title] = item
	}
	return out
}

const day = 24 * time.Hour

func TestExpireDue_HidesOnlyOldUnreadUnbookmarkedItems(t *testing.T) {
	client := newFeedsClient(t)
	feedID := seedExpiryFeed(t, 7)
	seedAgedItem(t, feedID, "old-unread", 8*day, "")
	seedAgedItem(t, feedID, "fresh-unread", 1*day, "")
	seedAgedItem(t, feedID, "old-read", 8*day, "read_at = now()")
	seedAgedItem(t, feedID, "old-bookmarked", 8*day, "bookmarked = true")
	seedAgedItem(t, feedID, "old-dismissed", 8*day, "dismissed = true")
	seedAgedItem(t, feedID, "old-restored", 8*day, "restored_at = now()")

	expired, err := testApp.Services.Feeds.ExpireDue(context.Background())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, expired, int64(1))

	inbox := titlesIn(t, client, feedID, &feedsv1.ListFeedItemsRequest{})
	assert.NotContains(t, inbox, "old-unread")
	for _, kept := range []string{
		"fresh-unread", "old-read", "old-bookmarked", "old-restored",
	} {
		assert.Contains(t, inbox, kept)
	}

	hidden := titlesIn(t, client, feedID,
		&feedsv1.ListFeedItemsRequest{ExpiredOnly: proto.Bool(true)})
	require.Len(t, hidden, 1)
	assert.NotEmpty(t, hidden["old-unread"].ExpiredAt)
}

func TestExpireDue_FeedThatNeverExpiresKeepsItems(t *testing.T) {
	client := newFeedsClient(t)
	feedID := seedExpiryFeed(t, 0)
	seedAgedItem(t, feedID, "ancient", 400*day, "")

	_, err := testApp.Services.Feeds.ExpireDue(context.Background())
	require.NoError(t, err)

	assert.Contains(t, titlesIn(t, client, feedID, &feedsv1.ListFeedItemsRequest{}),
		"ancient")
}

func TestRestoreFeedItem_ExpiredReturnsUnreadAndNeverExpiresAgain(t *testing.T) {
	client := newFeedsClient(t)
	feedID := seedExpiryFeed(t, 7)
	itemID := seedAgedItem(t, feedID, "old-unread", 8*day, "")
	_, err := testApp.Services.Feeds.ExpireDue(context.Background())
	require.NoError(t, err)

	restored := restoreItem(t, client, itemID)
	assert.Empty(t, restored.ExpiredAt)
	assert.NotEmpty(t, restored.RestoredAt)
	assert.Empty(t, restored.ReadAt)
	assert.Empty(t, restored.ExpiresAt)

	_, err = testApp.Services.Feeds.ExpireDue(context.Background())
	require.NoError(t, err)
	assert.Contains(t, titlesIn(t, client, feedID,
		&feedsv1.ListFeedItemsRequest{UnreadOnly: proto.Bool(true)}), "old-unread")
	assert.Empty(t, titlesIn(t, client, feedID,
		&feedsv1.ListFeedItemsRequest{ExpiredOnly: proto.Bool(true)}))
}

func TestExpiredItems_OtherUserCannotListOrRestore(t *testing.T) {
	client := newFeedsClient(t)
	var feedID, itemID string
	require.NoError(t, testDB.QueryRow(context.Background(), `
		INSERT INTO feeds.feeds (user_id, url, title, source_type)
		VALUES ($1, $2, 'Foreign', 'rss') RETURNING id
	`, "another-user-"+uuid.NewString(), uniqueBlogBase()+"/feed.xml").Scan(&feedID))
	require.NoError(t, testDB.QueryRow(context.Background(), `
		INSERT INTO feeds.items
		    (feed_id, guid, title, source_url, published_at, expired_at)
		VALUES ($1, 'g', 'Foreign item', 'https://example.com/f', now(), now())
		RETURNING id
	`, feedID).Scan(&itemID))

	assert.Empty(t, titlesIn(t, client, feedID,
		&feedsv1.ListFeedItemsRequest{ExpiredOnly: proto.Bool(true)}))
	_, err := client.RestoreFeedItem(context.Background(),
		&feedsv1.RestoreFeedItemRequest{ItemId: itemID})
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestListFeedItems_LastChanceShowsItemsExpiringWithin48h(t *testing.T) {
	client := newFeedsClient(t)
	feedID := seedExpiryFeed(t, 7)
	seedAgedItem(t, feedID, "fading", 6*day+12*time.Hour, "")
	seedAgedItem(t, feedID, "fresh", 1*day, "")
	seedAgedItem(t, feedID, "fading-but-read", 6*day+12*time.Hour, "read_at = now()")
	seedAgedItem(t, feedID, "fading-bookmarked", 6*day+12*time.Hour,
		"bookmarked = true")

	last := titlesIn(t, client, feedID,
		&feedsv1.ListFeedItemsRequest{LastChanceOnly: proto.Bool(true)})
	require.Len(t, last, 1)
	require.Contains(t, last, "fading")
	expires, err := time.Parse(time.RFC3339, last["fading"].ExpiresAt)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(12*time.Hour), expires, time.Minute)

	inbox := titlesIn(t, client, feedID, &feedsv1.ListFeedItemsRequest{})
	assert.NotEmpty(t, inbox["fading"].ExpiresAt)
	assert.Empty(t, inbox["fading-bookmarked"].ExpiresAt)
}

func TestUpdateFeed_ExpiryDays(t *testing.T) {
	client := newFeedsClient(t)
	feedID := seedExpiryFeed(t, 7)
	update := func(req *feedsv1.UpdateFeedRequest) error {
		req.FeedId = feedID
		_, err := client.UpdateFeed(context.Background(), req)
		return err
	}
	expiryDays := func() int32 {
		resp, err := client.ListFeeds(context.Background(), &feedsv1.ListFeedsRequest{})
		require.NoError(t, err)
		for _, f := range resp.Feeds {
			if f.Id == feedID {
				return f.ExpiryDays
			}
		}
		t.Fatal("feed not listed")
		return -1
	}
	require.EqualValues(t, 7, expiryDays())

	require.NoError(t, update(&feedsv1.UpdateFeedRequest{
		Title: "Renamed", ExpiryDays: proto.Int32(3),
	}))
	assert.EqualValues(t, 3, expiryDays())

	require.NoError(t, update(&feedsv1.UpdateFeedRequest{Title: "Renamed again"}))
	assert.EqualValues(t, 3, expiryDays(), "unset leaves the lifetime unchanged")

	require.NoError(t, update(&feedsv1.UpdateFeedRequest{
		Title: "Renamed again", ExpiryDays: proto.Int32(0),
	}))
	assert.EqualValues(t, 0, expiryDays())

	for _, bad := range []int32{-1, 3651} {
		err := update(&feedsv1.UpdateFeedRequest{
			Title: "x", ExpiryDays: proto.Int32(bad),
		})
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	}
}

func TestRunExpiryNow_ExpiresThroughTrackedJob(t *testing.T) {
	client := newFeedsClient(t)
	feedID := seedExpiryFeed(t, 7)
	seedAgedItem(t, feedID, "old-unread", 8*day, "")

	require.NoError(t, testApp.RunExpiryNow(context.Background()))

	assert.NotContains(t, titlesIn(t, client, feedID, &feedsv1.ListFeedItemsRequest{}),
		"old-unread")
}

func TestListTopFeedItems_CapsAtTenAndKeepsTodaysReadItems(t *testing.T) {
	ctx := context.Background()
	owner := "top-user-" + uuid.NewString()
	var feedID string
	require.NoError(t, testDB.QueryRow(ctx, `
		INSERT INTO feeds.feeds (user_id, url, title, source_type)
		VALUES ($1, $2, 'Top feed', 'rss') RETURNING id
	`, owner, uniqueBlogBase()+"/feed.xml").Scan(&feedID))
	var ids []string
	for i := range 14 {
		var id string
		require.NoError(t, testDB.QueryRow(ctx, `
			INSERT INTO feeds.items (feed_id, guid, title, source_url, published_at)
			VALUES ($1, $2, $2, 'https://example.com/x',
			        now() - make_interval(hours => $3))
			RETURNING id
		`, feedID, uuid.NewString(), i).Scan(&id))
		ids = append(ids, id)
	}

	top, err := testApp.Services.Feeds.ListTopItems(ctx, owner)
	require.NoError(t, err)
	require.Len(t, top, 10)

	// Reading one today keeps it in the list instead of pulling in an 11th.
	_, err = testDB.Exec(ctx,
		`UPDATE feeds.items SET read_at = now() WHERE id = $1`, top[0].ID)
	require.NoError(t, err)
	again, err := testApp.Services.Feeds.ListTopItems(ctx, owner)
	require.NoError(t, err)
	require.Len(t, again, 10)
	var found bool
	for _, item := range again {
		found = found || item.ID == top[0].ID
	}
	assert.True(t, found)

	// Read before today, it drops out of the candidates.
	_, err = testDB.Exec(ctx, `
		UPDATE feeds.items SET read_at = now() - interval '2 days' WHERE id = $1
	`, top[0].ID)
	require.NoError(t, err)
	later, err := testApp.Services.Feeds.ListTopItems(ctx, owner)
	require.NoError(t, err)
	for _, item := range later {
		assert.NotEqual(t, top[0].ID, item.ID)
	}
	assert.Len(t, ids, 14)
}

func TestListTopFeedItems_Handler(t *testing.T) {
	client := newFeedsClient(t)
	feedID := seedExpiryFeed(t, 0)
	seedAgedItem(t, feedID, "top-candidate", time.Hour, "")

	resp, err := client.ListTopFeedItems(context.Background(),
		&feedsv1.ListTopFeedItemsRequest{})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(resp.Items), 10)
}

func TestDigestItems_FailsOnCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := testApp.ListDigestItems(ctx)
	require.Error(t, err)
	_, err = testApp.Services.Feeds.ExpireDue(ctx)
	require.Error(t, err)
}
