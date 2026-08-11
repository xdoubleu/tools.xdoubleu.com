package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/icsproxy/internal/models"
	"tools.xdoubleu.com/apps/icsproxy/internal/repositories"
	"tools.xdoubleu.com/internal/safedial"
)

// newTestService builds a service whose fetcher may reach loopback, so unit
// tests can point it at httptest servers.
func newTestServiceWithStore(store calendarStore) *CalendarService {
	return &CalendarService{
		logger: nil,
		repo:   store,
		client: safedial.Client(calendarFetchTimeout, maxCalendarRedirects, true),
	}
}

// fakeCalendarStore implements calendarStore in memory for unit tests.
type fakeCalendarStore struct {
	cfg   models.FilterConfig
	found bool
}

func (f *fakeCalendarStore) UpsertFilterConfig(
	_ context.Context, _ models.FilterConfig,
) error {
	return nil
}

func (f *fakeCalendarStore) GetFilterConfig(
	_ context.Context, _ string,
) (models.FilterConfig, bool) {
	return f.cfg, f.found
}

func (f *fakeCalendarStore) ListFilterConfigs(
	_ context.Context, _ string, _, _ int32,
) ([]models.FilterConfig, bool, error) {
	return nil, false, nil
}

func (f *fakeCalendarStore) ListFilterSummaries(
	_ context.Context, _ string,
) ([]repositories.FilterSummary, error) {
	return nil, nil
}

func (f *fakeCalendarStore) DeleteFilterConfig(
	_ context.Context, _ string, _ string,
) error {
	return nil
}

func TestAuthorizeConfigAccess_NotFound(t *testing.T) {
	//nolint:exhaustruct // test fields only
	err := authorizeConfigAccess(models.FilterConfig{}, false, "user-1")
	assert.ErrorIs(t, err, ErrConfigNotFound)
}

func TestAuthorizeConfigAccess_WrongOwner(t *testing.T) {
	//nolint:exhaustruct // test fields only
	cfg := models.FilterConfig{UserID: "owner"}
	err := authorizeConfigAccess(cfg, true, "someone-else")
	assert.ErrorIs(t, err, ErrConfigAccessDenied)
}

func TestAuthorizeConfigAccess_Owner(t *testing.T) {
	//nolint:exhaustruct // test fields only
	cfg := models.FilterConfig{UserID: "owner"}
	err := authorizeConfigAccess(cfg, true, "owner")
	assert.NoError(t, err)
}

func TestGetConfigWithEvents_NotFound(t *testing.T) {
	//nolint:exhaustruct // found only
	svc := newTestServiceWithStore(&fakeCalendarStore{found: false})

	_, _, err := svc.GetConfigWithEvents(t.Context(), "missing-token", "user-1")
	assert.ErrorIs(t, err, ErrConfigNotFound)
}

func TestGetConfigWithEvents_AccessDenied(t *testing.T) {
	store := &fakeCalendarStore{
		found: true,
		//nolint:exhaustruct // only UserID matters for the ownership check
		cfg: models.FilterConfig{UserID: "owner"},
	}
	svc := newTestServiceWithStore(store)

	_, _, err := svc.GetConfigWithEvents(t.Context(), "token", "someone-else")
	assert.ErrorIs(t, err, ErrConfigAccessDenied)
}

func TestGetConfigWithEvents_FetchError(t *testing.T) {
	store := &fakeCalendarStore{
		found: true,
		//nolint:exhaustruct // SourceURL/UserID only
		cfg: models.FilterConfig{UserID: "owner", SourceURL: "http://127.0.0.1:1"},
	}
	svc := newTestServiceWithStore(store)

	_, _, err := svc.GetConfigWithEvents(t.Context(), "token", "owner")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrConfigNotFound)
	assert.NotErrorIs(t, err, ErrConfigAccessDenied)
}

func TestGetConfigWithEvents_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/calendar")
			_, _ = w.Write(buildICS(
				vevent("uid-1", "Team Meeting", "20240115T090000Z", "20240115T100000Z"),
			))
		},
	))
	defer srv.Close()

	store := &fakeCalendarStore{
		found: true,
		//nolint:exhaustruct // SourceURL/UserID only
		cfg: models.FilterConfig{UserID: "owner", SourceURL: srv.URL},
	}
	svc := newTestServiceWithStore(store)

	cfg, events, err := svc.GetConfigWithEvents(t.Context(), "token", "owner")
	require.NoError(t, err)
	assert.Equal(t, "owner", cfg.UserID)
	require.Len(t, events, 1)
	assert.Equal(t, "uid-1", events[0].UID)
}
