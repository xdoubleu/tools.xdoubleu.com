//nolint:exhaustruct // sparse test fixtures
package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/internal/consumption"
	"tools.xdoubleu.com/apps/taste/internal/models"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/kit/logging"
)

type fakeFamily struct {
	id  uuid.UUID
	err error
}

func (f fakeFamily) EnsureFamily(context.Context, string) (uuid.UUID, error) {
	return f.id, f.err
}

type allowApps []string

func (a allowApps) CanAccess(_ context.Context, name string) bool {
	for _, n := range a {
		if n == name {
			return true
		}
	}
	return false
}

func TestScope_SoloIsTheCallerWithTheirApps(t *testing.T) {
	s := NewScopeService(fakeFamily{err: errors.New("unused")}, allowApps{"books"})
	sc, err := s.Resolve(context.Background(), "u1", false)
	require.NoError(t, err)
	assert.Equal(t, "u1", sc.User)
	assert.Equal(t, "u1", sc.Subject)
	assert.True(t, sc.Access("books"))
	assert.False(t, sc.Access("games"))
}

func TestScope_SharedIsTheFamilyAndOnlyMovies(t *testing.T) {
	fam := uuid.New()
	s := NewScopeService(fakeFamily{id: fam}, allowApps{"books", SharedApp})
	sc, err := s.Resolve(context.Background(), "u1", true)
	require.NoError(t, err)
	assert.Equal(t, "u1", sc.User)
	assert.Equal(t, consumption.SharedSubject(fam), sc.Subject)
	assert.True(t, sc.Access(SharedApp))
	assert.False(t, sc.Access("books"))

	denied := NewScopeService(fakeFamily{id: fam}, allowApps{"books"})
	sc, err = denied.Resolve(context.Background(), "u1", true)
	require.NoError(t, err)
	assert.False(t, sc.Access(SharedApp))
}

func TestScope_SharedFamilyErrorFails(t *testing.T) {
	boom := errors.New("boom")
	s := NewScopeService(fakeFamily{err: boom}, allowApps{SharedApp})
	_, err := s.Resolve(context.Background(), "u1", true)
	require.ErrorIs(t, err, boom)
}

func openScope(apps ...string) Scope {
	return Scope{
		User: "u", Subject: "u",
		Access: func(name string) bool {
			return allowApps(apps).CanAccess(context.Background(), name)
		},
	}
}

func eventsOf(n int) []consumption.Event {
	out := make([]consumption.Event, n)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range out {
		//nolint:exhaustruct // dated books are enough
		out[i] = consumption.Event{
			Domain: consumption.DomainBooks, Title: string(rune('a' + i)),
			At: base.Add(-time.Duration(i) * time.Hour),
		}
	}
	return out
}

func TestReads_EventsPaginate(t *testing.T) {
	src := fakeBooks{events: eventsOf(5)}
	r := NewReadService(NewLogService([]Source{src}), &recStore{})
	sc := openScope("books")

	page, more, err := r.Events(context.Background(), sc, "", 2, 1)
	require.NoError(t, err)
	require.Len(t, page, 2)
	assert.Equal(t, "b", page[0].Title)
	assert.True(t, more)

	page, more, err = r.Events(context.Background(), sc, consumption.DomainBooks, 10, 4)
	require.NoError(t, err)
	assert.Len(t, page, 1)
	assert.False(t, more)

	page, more, err = r.Events(context.Background(), sc, "", 10, 99)
	require.NoError(t, err)
	assert.Empty(t, page)
	assert.False(t, more)
}

func TestReads_EventsRejectBadInput(t *testing.T) {
	r := NewReadService(NewLogService(nil), &recStore{})
	_, _, err := r.Events(context.Background(), openScope(), "music", 0, 0)
	var httpErr *app.HTTPError
	require.ErrorAs(t, err, &httpErr)
	_, _, err = r.Events(context.Background(), openScope(), "", 0, -1)
	require.ErrorAs(t, err, &httpErr)
}

func TestReads_StatsKeepsHitRatesOfOpenApps(t *testing.T) {
	store := &recStore{rates: []models.HitRate{
		{Domain: "books", Track: "more", Hits: 1, Misses: 2},
		{Domain: "games", Track: "more", Hits: 3, Misses: 4},
	}}
	r := NewReadService(NewLogService(nil), store)
	_, rates, err := r.Stats(context.Background(), openScope("books"))
	require.NoError(t, err)
	require.Len(t, rates, 1)
	assert.Equal(t, "books", rates[0].Domain)

	store.listErr = errors.New("boom")
	_, _, err = r.Stats(context.Background(), openScope("books"))
	require.Error(t, err)
}

type fakeBooks struct{ events []consumption.Event }

func (fakeBooks) GetName() string { return "books" }

func (f fakeBooks) ListConsumption(
	context.Context, string,
) ([]consumption.Event, error) {
	return f.events, nil
}

// domainStore serves picks of fixed domains.
type domainStore struct {
	*recStore
	domains []string
}

func (d domainStore) List(
	context.Context, string, string, string, bool,
) ([]models.Recommendation, error) {
	out := make([]models.Recommendation, len(d.domains))
	for i, dom := range d.domains {
		//nolint:exhaustruct // the domain is what is filtered on
		out[i] = models.Recommendation{Domain: dom}
	}
	return out, nil
}

func (d domainStore) Status(context.Context, string) (models.PicksStatus, error) {
	return models.PicksStatus{Error: "note"}, nil
}

func (d domainStore) Get(
	_ context.Context, _, id string,
) (*models.Recommendation, error) {
	//nolint:exhaustruct // the id and domain are enough
	return &models.Recommendation{ID: id, Domain: d.domains[0]}, nil
}

func newPicks(store recommendationStore) *RecommendService {
	return NewRecommendService(
		NewLogService(nil), nil, store, Catalogs{}, nil, "m", logging.NewNopLogger(),
	)
}

func TestPicks_VisibleFiltersByAccessAndValidates(t *testing.T) {
	s := newPicks(domainStore{recStore: &recStore{}, domains: []string{"books", "games"}})
	recs, st, err := s.Visible(
		context.Background(), openScope("books"), "", "", false,
	)
	require.NoError(t, err)
	require.Len(t, recs, 1)
	assert.Equal(t, "books", recs[0].Domain)
	assert.Equal(t, "note", st.Error)

	var httpErr *app.HTTPError
	_, _, err = s.Visible(context.Background(), openScope(), "music", "", false)
	require.ErrorAs(t, err, &httpErr)
	_, _, err = s.Visible(context.Background(), openScope(), "", "sideways", false)
	require.ErrorAs(t, err, &httpErr)
}

func TestPicks_OwnHidesPicksOfClosedApps(t *testing.T) {
	s := newPicks(domainStore{recStore: &recStore{}, domains: []string{"games"}})
	id := uuid.NewString()

	rec, err := s.Own(context.Background(), openScope("games"), id)
	require.NoError(t, err)
	assert.Equal(t, id, rec.ID)

	_, err = s.Own(context.Background(), openScope("books"), id)
	require.ErrorIs(t, err, ErrPickNotFound)

	var httpErr *app.HTTPError
	_, err = s.Own(context.Background(), openScope("games"), "nope")
	require.ErrorAs(t, err, &httpErr)
}

func TestPicks_SetFeedbackRejectsUnknownFeedback(t *testing.T) {
	s := newPicks(&recStore{})
	var httpErr *app.HTTPError
	_, err := s.SetFeedback(context.Background(), "u", "id", "love")
	require.ErrorAs(t, err, &httpErr)
	_, err = s.SetFeedback(context.Background(), "u", "id", models.FeedbackHit)
	require.NoError(t, err)
}
