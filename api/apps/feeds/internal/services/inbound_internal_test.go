package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/feeds/internal/models"
	"tools.xdoubleu.com/internal/kit/database"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// fakeFeeds serves email feeds by token hash and records fetch results; the
// embedded nil interface panics on any method a test doesn't expect.
type fakeFeeds struct {
	feedsStore
	byHash   map[string]models.Feed
	inserted []models.Feed
	results  []*string
}

func (f *fakeFeeds) GetByInboundTokenHash(
	_ context.Context, hash string,
) (*models.Feed, error) {
	feed, ok := f.byHash[hash]
	if !ok {
		return nil, database.ErrResourceNotFound
	}
	return &feed, nil
}

func (f *fakeFeeds) Insert(
	_ context.Context, feed models.Feed,
) (*models.Feed, error) {
	feed.ID = uuid.New()
	f.inserted = append(f.inserted, feed)
	return &feed, nil
}

func (f *fakeFeeds) SetFetchResult(
	_ context.Context, id uuid.UUID, _, _, fetchErr *string,
) (*models.Feed, error) {
	f.results = append(f.results, fetchErr)
	//nolint:exhaustruct // only the fields under test
	return &models.Feed{ID: id, LastError: fetchErr}, nil
}

type fakeItems struct {
	itemsStore
	inserted []models.Item
}

func (f *fakeItems) Insert(_ context.Context, item models.Item) error {
	f.inserted = append(f.inserted, item)
	return nil
}

func (f *fakeItems) RecentPublishedAt(
	context.Context, uuid.UUID, int,
) ([]time.Time, error) {
	return nil, nil
}

type fakeRules struct{ filterRulesStore }

func (fakeRules) ListForFeed(
	context.Context, string, uuid.UUID,
) ([]models.FilterRule, error) {
	return nil, nil
}

type fakeMail struct {
	html string
	err  error
	ids  []string
}

func (m *fakeMail) HTML(_ context.Context, emailID string) (string, error) {
	m.ids = append(m.ids, emailID)
	return m.html, m.err
}

func tokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

type inboundFixture struct {
	svc   *FeedService
	feeds *fakeFeeds
	items *fakeItems
	mail  *fakeMail
	feed  models.Feed
}

func newInboundFixture() *inboundFixture {
	//nolint:exhaustruct // only the fields under test
	feed := models.Feed{ID: uuid.New(), UserID: "u1", SourceType: models.FeedSourceEmail}
	//nolint:exhaustruct // fakes record into their zero-valued slices
	feeds := &fakeFeeds{byHash: map[string]models.Feed{tokenHash("abc123"): feed}}
	//nolint:exhaustruct // fakes record into their zero-valued slices
	items := &fakeItems{}
	//nolint:exhaustruct // fakes record into their zero-valued slices
	mail := &fakeMail{html: "<p>hello</p>"}
	svc := NewFeedService(
		slog.Default(), nil, feeds, items, fakeRules{filterRulesStore: nil}, nil, "mail.example.com",
		nil, nil, "", mail,
	)
	return &inboundFixture{svc: svc, feeds: feeds, items: items, mail: mail, feed: feed}
}

func TestHandleInboundEmail_IngestsItemForKnownAlias(t *testing.T) {
	fx := newInboundFixture()

	//nolint:exhaustruct // only the fields under test
	fx.svc.HandleInboundEmail(context.Background(), InboundEmail{
		EmailID: "e1", MessageID: "m1", Subject: "Hi",
		To: []string{"reading+abc123@mail.example.com"},
	})

	require.Len(t, fx.items.inserted, 1)
	item := fx.items.inserted[0]
	assert.Equal(t, fx.feed.ID, item.FeedID)
	assert.Equal(t, "Hi", item.Title)
	assert.Equal(t, "<p>hello</p>", item.ContentHTML)
	assert.Equal(t, "mailto:"+fx.feed.ID.String()+"/m1", item.GUID)
	assert.Equal(t, []string{"e1"}, fx.mail.ids)
}

func TestHandleInboundEmail_MissingMessageIDFallsBackToEmailID(t *testing.T) {
	fx := newInboundFixture()

	//nolint:exhaustruct // only the fields under test
	fx.svc.HandleInboundEmail(context.Background(), InboundEmail{
		EmailID: "e1", To: []string{"abc123@mail.example.com"},
	})

	require.Len(t, fx.items.inserted, 1)
	assert.Equal(t, "mailto:"+fx.feed.ID.String()+"/e1", fx.items.inserted[0].GUID)
}

func TestHandleInboundEmail_BlankSubjectDefaultsTitle(t *testing.T) {
	fx := newInboundFixture()

	//nolint:exhaustruct // only the fields under test
	fx.svc.HandleInboundEmail(context.Background(), InboundEmail{
		EmailID: "e1", To: []string{"abc123@mail.example.com"},
	})

	require.Len(t, fx.items.inserted, 1)
	assert.Equal(t, "Email newsletter", fx.items.inserted[0].Title)
}

func TestHandleInboundEmail_ReceivedForFallback(t *testing.T) {
	fx := newInboundFixture()

	//nolint:exhaustruct // only the fields under test
	fx.svc.HandleInboundEmail(context.Background(), InboundEmail{
		EmailID:     "e1",
		To:          []string{"someone@else.example.com", "not-an-address"},
		ReceivedFor: []string{"abc123@mail.example.com"},
	})

	assert.Len(t, fx.items.inserted, 1)
}

func TestHandleInboundEmail_UnknownAliasIsANoOp(t *testing.T) {
	fx := newInboundFixture()

	//nolint:exhaustruct // only the fields under test
	fx.svc.HandleInboundEmail(context.Background(), InboundEmail{
		EmailID: "e1", To: []string{"unknown@mail.example.com"},
	})

	assert.Empty(t, fx.items.inserted)
	assert.Empty(t, fx.mail.ids, "no body is fetched for an unknown alias")
}

func TestHandleInboundEmail_FetchFailureIsRecordedOnTheFeed(t *testing.T) {
	fx := newInboundFixture()
	fx.mail.err = errors.New("resend down")

	//nolint:exhaustruct // only the fields under test
	fx.svc.HandleInboundEmail(context.Background(), InboundEmail{
		EmailID: "e1", To: []string{"abc123@mail.example.com"},
	})

	assert.Empty(t, fx.items.inserted)
	require.Len(t, fx.feeds.results, 1)
	require.NotNil(t, fx.feeds.results[0])
	assert.Equal(t, "resend down", *fx.feeds.results[0])
}

func TestHandleInboundEmail_WithoutMailPortRecordsFailure(t *testing.T) {
	fx := newInboundFixture()
	fx.svc.mail = nil

	//nolint:exhaustruct // only the fields under test
	fx.svc.HandleInboundEmail(context.Background(), InboundEmail{
		EmailID: "e1", To: []string{"abc123@mail.example.com"},
	})

	assert.Empty(t, fx.items.inserted)
	require.Len(t, fx.feeds.results, 1)
	assert.NotNil(t, fx.feeds.results[0])
}

func TestCreateFeed_EmailWithURLIsRejected(t *testing.T) {
	fx := newInboundFixture()

	_, _, err := fx.svc.CreateFeed(context.Background(), "u1", NewFeed{
		Kind: models.FeedSourceEmail, URL: "https://example.com", Title: "t",
	})

	require.ErrorIs(t, err, ErrEmailFeedHasURL)
	assert.Empty(t, fx.feeds.inserted)
}

func TestCreateFeed_EmailMintsAnAddress(t *testing.T) {
	fx := newInboundFixture()

	feed, address, err := fx.svc.CreateFeed(context.Background(), "u1", NewFeed{
		Kind: models.FeedSourceEmail, URL: "", Title: "News",
	})

	require.NoError(t, err)
	assert.Equal(t, "News", feed.Title)
	assert.Equal(t, models.FeedSourceEmail, feed.SourceType)
	token, ok := inboundTokenFromAddress(address)
	require.True(t, ok)
	require.Len(t, fx.feeds.inserted, 1)
	assert.Equal(t, tokenHash(token), *fx.feeds.inserted[0].InboundToken)
}

func TestCreateFeed_EmailNeedsInboundDomain(t *testing.T) {
	fx := newInboundFixture()
	fx.svc.inboundDomain = ""

	_, _, err := fx.svc.CreateFeed(context.Background(), "u1", NewFeed{
		Kind: models.FeedSourceEmail, URL: "", Title: "",
	})

	require.ErrorIs(t, err, ErrEmailFeedsNotConfigured)
}

func TestListUnhealthyAs_OnlyAdmins(t *testing.T) {
	fx := newInboundFixture()
	fx.svc.feeds = unhealthyFeeds{feedsStore: fx.feeds}

	_, err := fx.svc.ListUnhealthyAs(context.Background(), nil)
	require.ErrorIs(t, err, ErrAdminRequired)

	//nolint:exhaustruct // only Role matters
	_, err = fx.svc.ListUnhealthyAs(
		context.Background(), &sharedmodels.User{Role: sharedmodels.RoleUser},
	)
	require.ErrorIs(t, err, ErrAdminRequired)

	//nolint:exhaustruct // only Role matters
	feeds, err := fx.svc.ListUnhealthyAs(
		context.Background(), &sharedmodels.User{Role: sharedmodels.RoleAdmin},
	)
	require.NoError(t, err)
	assert.Len(t, feeds, 1)
}

type unhealthyFeeds struct{ feedsStore }

func (unhealthyFeeds) ListUnhealthy(context.Context) ([]models.Feed, error) {
	//nolint:exhaustruct // only the fields under test
	return []models.Feed{{ID: uuid.New()}}, nil
}

func TestInboundTokenFromAddress(t *testing.T) {
	tests := []struct {
		name      string
		addr      string
		wantToken string
		wantOK    bool
	}{
		{"valid alias", "reading+abc123@mail.example.com", "abc123", true},
		// No "+" required: whole local part is the token.
		{"no plus", "abc123@mail.example.com", "abc123", true},
		{"no at", "reading+abc123", "", false},
		{"empty token after plus", "reading+@mail.example.com", "", false},
		{"empty local part", "@mail.example.com", "", false},
		// Tokens are case-folded, surviving relays that lowercase.
		{
			"mixed case folded to lowercase",
			"reading+AbC123@mail.example.com",
			"abc123",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, ok := inboundTokenFromAddress(tt.addr)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantToken, token)
		})
	}
}
