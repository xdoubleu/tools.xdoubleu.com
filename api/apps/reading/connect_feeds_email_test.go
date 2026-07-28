package reading_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
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
) (booksTestClient, *mocks.MockWebFetchClient) {
	t.Helper()
	app, webFetch := newEmailConfiguredApp(t, "whsec_dGVzdA==", "mail.test.example")
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
	client, _ := newEmailConfiguredTestClient(t)

	resp, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{
			Kind: readingv1.FeedKind_FEED_KIND_EMAIL,
		}),
	)
	require.NoError(t, err)

	assert.Equal(t, "email", resp.Msg.Feed.SourceType)
	assert.True(t, strings.HasPrefix(resp.Msg.Feed.InboundAddress, "reading+"))
	assert.True(
		t,
		strings.HasSuffix(resp.Msg.Feed.InboundAddress, "@mail.test.example"),
	)

	// The token must be lowercase hex (issue #661) — an alphabet with no
	// case ambiguity for a mail relay to mangle in transit.
	token := strings.TrimSuffix(
		strings.TrimPrefix(resp.Msg.Feed.InboundAddress, "reading+"),
		"@mail.test.example",
	)
	assert.Regexp(t, "^[0-9a-f]{64}$", token)

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

// TestCreateFeed_Email_CustomTitle proves that a title passed on
// CreateFeed(kind=EMAIL) is stored on the feed (issue #629 — newsletters
// were always hardcoded to "Email newsletter" with no way to name them).
func TestCreateFeed_Email_CustomTitle(t *testing.T) {
	client, _ := newEmailConfiguredTestClient(t)

	resp, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{
			Kind:  readingv1.FeedKind_FEED_KIND_EMAIL,
			Title: "My Substack",
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, "My Substack", resp.Msg.Feed.Title)
}

// TestCreateFeed_Email_DefaultTitle proves that an empty title still falls
// back to "Email newsletter" rather than storing a blank title.
func TestCreateFeed_Email_DefaultTitle(t *testing.T) {
	client, _ := newEmailConfiguredTestClient(t)

	resp, err := client.CreateFeed(
		context.Background(),
		feedReq(
			t,
			&readingv1.CreateFeedRequest{Kind: readingv1.FeedKind_FEED_KIND_EMAIL},
		),
	)
	require.NoError(t, err)
	assert.Equal(t, "Email newsletter", resp.Msg.Feed.Title)
}

// TestCreateFeed_Email_GenericErrorMapped proves that a CreateEmail failure
// other than ErrEmailFeedsNotConfigured (e.g. a DB error) is mapped through
// feedErrorToConnect rather than always being read as "not configured". Uses
// an app wired to an unreachable DB (port 1, immediate connection refusal)
// so the Insert call genuinely fails — everything before it (auth, the
// inboundDomain check) still succeeds normally.
func TestCreateFeed_Email_GenericErrorMapped(t *testing.T) {
	brokenDB, err := pgxpool.New(context.Background(), "postgres://u:p@127.0.0.1:1/db")
	require.NoError(t, err)
	t.Cleanup(brokenDB.Close)

	cfg := testCfg
	cfg.EmailInboundDomain = "mail.test.example"
	app := reading.NewInner(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		cfg,
		brokenDB,
		//nolint:exhaustruct // unused by CreateEmail's DB-only failure path
		reading.Clients{},
	)
	ts := httptest.NewServer(testhelper.BuildMux(app))
	t.Cleanup(ts.Close)
	client := newBooksClientFor(ts.URL, connect.WithHTTPGet())

	_, err = client.CreateFeed(
		context.Background(),
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
	client, webFetch := newEmailConfiguredTestClient(t)

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
