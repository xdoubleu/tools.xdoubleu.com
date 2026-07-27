package reading_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/reading"
	"tools.xdoubleu.com/apps/reading/internal/mocks"
	"tools.xdoubleu.com/apps/reading/pkg/objectstore"
	readingv1 "tools.xdoubleu.com/gen/reading/v1"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

// newEmailConfiguredApp builds a Reading app sharing testDB but with
// EMAIL_INBOUND_SECRET/EMAIL_INBOUND_DOMAIN set, for email-feed tests that
// need those configured (testApp deliberately leaves them unset — see
// TestCreateFeed_Email_NotConfigured).
func newEmailConfiguredApp(
	t *testing.T,
	secret, domain string,
) (*reading.Reading, *mocks.MockWebFetchClient) {
	t.Helper()
	cfg := testCfg
	cfg.EmailInboundSecret = secret
	cfg.EmailInboundDomain = domain

	webFetch := mocks.NewMockWebFetchClient()
	clients := reading.Clients{
		UniCat:           nil,
		Hardcover:        mocks.NewMockHardcoverClient(),
		ObjectStore:      objectstore.NewFake(),
		WebFetch:         webFetch,
		Arxiv:            mocks.NewMockArxivClient(),
		HTMLConvert:      nil,
		KoboStoreBaseURL: "",
		PublicAPIBaseURL: "",
	}
	app := reading.NewInner(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		cfg,
		testDB,
		clients,
	)
	return app, webFetch
}

func newEmailConfiguredTestClient(
	t *testing.T,
	secret, domain string,
) (booksTestClient, *mocks.MockWebFetchClient) {
	t.Helper()
	app, webFetch := newEmailConfiguredApp(t, secret, domain)
	ts := httptest.NewServer(testhelper.BuildMux(app))
	t.Cleanup(ts.Close)
	return newBooksClientFor(ts.URL, connect.WithHTTPGet()), webFetch
}

// TestCreateFeed_Email_NotConfigured proves that CreateEmail refuses to mint
// an inbound address when EMAIL_INBOUND_DOMAIN is unset — testApp's config
// never sets it, so every email feed created against it must fail rather
// than silently minting a dead address.
func TestCreateFeed_Email_NotConfigured(t *testing.T) {
	client := newBooksTestClient(t)
	_, err := client.CreateFeed(
		context.Background(),
		feedReq(
			t,
			&readingv1.CreateFeedRequest{Kind: readingv1.FeedKind_FEED_KIND_EMAIL},
		),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

// TestCreateFeed_Email_RejectsURL proves an email feed request carrying a
// url is rejected up front — url only means something for rss feeds.
func TestCreateFeed_Email_RejectsURL(t *testing.T) {
	client := newBooksTestClient(t)
	_, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{
			Kind: readingv1.FeedKind_FEED_KIND_EMAIL,
			Url:  "https://example.com/feed.xml",
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// TestCreateFeed_Email_MintsInboundAddress proves that, configured with an
// inbound domain, CreateFeed(kind=EMAIL) mints a "reading+<token>@domain"
// address and marks the feed source_type "email".
func TestCreateFeed_Email_MintsInboundAddress(t *testing.T) {
	client, _ := newEmailConfiguredTestClient(t, "whsec_dGVzdA==", "mail.test.example")

	resp, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{
			Kind:     readingv1.FeedKind_FEED_KIND_EMAIL,
			KoboSync: true,
		}),
	)
	require.NoError(t, err)

	assert.Equal(t, "email", resp.Msg.Feed.SourceType)
	assert.True(t, strings.HasPrefix(resp.Msg.Feed.InboundAddress, "reading+"))
	assert.True(
		t,
		strings.HasSuffix(resp.Msg.Feed.InboundAddress, "@mail.test.example"),
	)
	assert.True(t, resp.Msg.Feed.KoboSync)

	// ListFeeds must never re-expose the inbound address in plaintext.
	list, err := client.ListFeeds(
		context.Background(), feedReq(t, &readingv1.ListFeedsRequest{}),
	)
	require.NoError(t, err)
	for _, f := range list.Msg.Feeds {
		if f.Id == resp.Msg.Feed.Id {
			assert.Empty(t, f.InboundAddress)
		}
	}
}

// TestCreateFeed_Email_GenericErrorMapped proves that a CreateEmail failure
// other than ErrEmailFeedsNotConfigured (e.g. a DB error) is mapped through
// feedErrorToConnect rather than always being read as "not configured".
func TestCreateFeed_Email_GenericErrorMapped(t *testing.T) {
	client, _ := newEmailConfiguredTestClient(t, "whsec_dGVzdA==", "mail.test.example")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.CreateFeed(
		ctx,
		feedReq(
			t,
			&readingv1.CreateFeedRequest{Kind: readingv1.FeedKind_FEED_KIND_EMAIL},
		),
	)
	require.Error(t, err)
	assert.NotEqual(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

// TestRefreshFeed_EmailFeed_NoOp proves that RefreshFeed is a no-op for an
// email feed — it is push-only (Resend webhook), so polling it must never
// touch the web-fetch client.
func TestRefreshFeed_EmailFeed_NoOp(t *testing.T) {
	client, webFetch := newEmailConfiguredTestClient(
		t,
		"whsec_dGVzdA==",
		"mail.test.example",
	)

	created, err := client.CreateFeed(
		context.Background(),
		feedReq(
			t,
			&readingv1.CreateFeedRequest{Kind: readingv1.FeedKind_FEED_KIND_EMAIL},
		),
	)
	require.NoError(t, err)

	resp, err := client.RefreshFeed(
		context.Background(),
		feedReq(t, &readingv1.RefreshFeedRequest{FeedId: created.Msg.Feed.Id}),
	)
	require.NoError(t, err)
	assert.Equal(t, int32(0), resp.Msg.Ingested)
	assert.Empty(t, webFetch.Calls,
		"refreshing an email feed must never poll the web")
}
