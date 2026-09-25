package feeds_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/feeds"
	"tools.xdoubleu.com/apps/feeds/internal/mocks"
	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
	"tools.xdoubleu.com/gen/feeds/v1/feedsv1connect"
	"tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mailer"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/notifications"
	"tools.xdoubleu.com/internal/testhelper"
)

// capturingMailServer stands in for Resend and records every send.
type capturingMailServer struct {
	*httptest.Server
	mu       sync.Mutex
	sends    []map[string]any
	failNext bool
}

func newCapturingMailServer(t *testing.T) *capturingMailServer {
	t.Helper()
	c := &capturingMailServer{} //nolint:exhaustruct // Server set below
	c.Server = httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			c.mu.Lock()
			fail := c.failNext
			c.failNext = false
			if !fail {
				c.sends = append(c.sends, payload)
			}
			c.mu.Unlock()
			if fail {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		},
	))
	t.Cleanup(c.Close)
	return c
}

func (c *capturingMailServer) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.sends)
}

// FailNextSend makes the next send request fail with a 500, once.
func (c *capturingMailServer) FailNextSend() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failNext = true
}

// newNotifyTestApp builds a feeds app on testDB with its own webfetch mock
// and a mailer pointed at a capturing server. It returns the
// notifications.Service to wait on delivery and a buffer of FeedService logs.
func newNotifyTestApp(
	t *testing.T,
) (
	feedsv1connect.FeedServiceClient,
	*mocks.MockWebFetchClient,
	*capturingMailServer,
	*notifications.Service,
	*bytes.Buffer,
) {
	t.Helper()

	mailSrv := newCapturingMailServer(t)
	mailer.SetBaseURL(mailSrv.URL)
	t.Cleanup(func() { mailer.SetBaseURL("https://api.resend.com") })

	cfg := testhelper.NewTestConfig()
	cfg.EmailInboundDomain = "mail.example.com"
	cfg.ResendAPIKey = "test-resend-key"

	webFetch := mocks.NewMockWebFetchClient()
	var logBuf bytes.Buffer
	logger := slog.New(logging.NewBufLogHandler(&logBuf, nil))
	notifSvc := notifications.New(
		t.Context(),
		logging.NewNopLogger(),
		mailer.New("test-resend-key", "feeds@example.com", ""),
	)
	app := feeds.NewInner(
		sharedmocks.NewMockedAuthService(userID),
		logger,
		cfg,
		testDB,
		webFetch,
		notifSvc,
		appUsersRepo,
	)

	ts := httptest.NewServer(testhelper.BuildMux(app))
	t.Cleanup(ts.Close)
	client := feedsv1connect.NewFeedServiceClient(http.DefaultClient, ts.URL)
	return client, webFetch, mailSrv, notifSvc, &logBuf
}

func feedConsecutiveFailuresAndNotified(
	t *testing.T,
	feedID string,
) (int, bool) {
	t.Helper()
	var failures int
	var notifiedAt *time.Time
	err := testDB.QueryRow(
		context.Background(),
		"SELECT consecutive_failures, notified_at FROM feeds.feeds WHERE id = $1",
		feedID,
	).Scan(&failures, &notifiedAt)
	require.NoError(t, err)
	return failures, notifiedAt != nil
}

// TestFeedNotify_ErrorThreshold_DedupAndRecovery: no email before 3
// failures, exactly one while broken, marker cleared on recovery.
func TestFeedNotify_ErrorThreshold_DedupAndRecovery(t *testing.T) {
	client, webFetch, mailSrv, notifSvc, _ := newNotifyTestApp(t)

	base := uniqueBlogBase()
	feedURL := base + "/feed.xml"
	webFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"Flaky Blog", rssItem{"Post One", base + "/one", "guid-1", itemContent},
	)))

	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{Url: feedURL}),
	)
	require.NoError(t, err)
	feedID := created.Msg.Feed.Id
	waitForFeedImport(t, client, feedID)
	assert.Equal(t, 0, mailSrv.count())

	webFetch.Errs[feedURL] = errors.New("simulated fetch failure")

	for i := 0; i < 2; i++ {
		_, refreshErr := client.RefreshFeed(
			context.Background(),
			connect.NewRequest(&feedsv1.RefreshFeedRequest{FeedId: feedID}),
		)
		require.Error(t, refreshErr)
	}
	notifSvc.WaitUntilDone()
	assert.Equal(t, 0, mailSrv.count(), "no email before the 3rd consecutive failure")

	_, err = client.RefreshFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.RefreshFeedRequest{FeedId: feedID}),
	)
	require.Error(t, err)
	notifSvc.WaitUntilDone()
	assert.Equal(t, 1, mailSrv.count(), "3rd consecutive failure sends the alert")
	failures, notified := feedConsecutiveFailuresAndNotified(t, feedID)
	assert.Equal(t, 3, failures)
	assert.True(t, notified)

	_, err = client.RefreshFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.RefreshFeedRequest{FeedId: feedID}),
	)
	require.Error(t, err)
	notifSvc.WaitUntilDone()
	assert.Equal(t, 1, mailSrv.count(), "dedup: no 2nd email while still broken")

	delete(webFetch.Errs, feedURL)
	_, err = client.RefreshFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.RefreshFeedRequest{FeedId: feedID}),
	)
	require.NoError(t, err)
	notifSvc.WaitUntilDone()
	failures, notified = feedConsecutiveFailuresAndNotified(t, feedID)
	assert.Equal(t, 0, failures)
	assert.False(t, notified, "notified marker clears on recovery")
	assert.Equal(t, 1, mailSrv.count(), "recovery sends no further email")
}

// TestFeedNotify_SendFailure_LoggedAndNotMarkedNotified: a failed send is
// logged and not marked notified, so it retries next poll.
func TestFeedNotify_SendFailure_LoggedAndNotMarkedNotified(t *testing.T) {
	client, webFetch, mailSrv, notifSvc, logBuf := newNotifyTestApp(t)

	base := uniqueBlogBase()
	feedURL := base + "/feed.xml"
	webFetch.SetBody(feedURL, "application/rss+xml", []byte(rssXML(
		"Flaky Blog Two", rssItem{"Post One", base + "/one", "guid-1", itemContent},
	)))

	created, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{Url: feedURL}),
	)
	require.NoError(t, err)
	feedID := created.Msg.Feed.Id
	waitForFeedImport(t, client, feedID)

	webFetch.Errs[feedURL] = errors.New("simulated fetch failure")
	mailSrv.FailNextSend()

	for i := 0; i < 3; i++ {
		_, refreshErr := client.RefreshFeed(
			context.Background(),
			connect.NewRequest(&feedsv1.RefreshFeedRequest{FeedId: feedID}),
		)
		require.Error(t, refreshErr)
	}
	notifSvc.WaitUntilDone()

	assert.Equal(t, 0, mailSrv.count(), "the failed send is never recorded as sent")
	assert.Contains(t, logBuf.String(), "feed notify: send failed")
	_, notified := feedConsecutiveFailuresAndNotified(t, feedID)
	assert.False(t, notified, "a failed send must not mark the feed as notified")
}
