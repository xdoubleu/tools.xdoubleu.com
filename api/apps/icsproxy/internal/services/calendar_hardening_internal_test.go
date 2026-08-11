package services

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/icsproxy/internal/models"
	"tools.xdoubleu.com/internal/safedial"
)

// ── FetchICS ─────────────────────────────────────────────────────────────────

func TestFetchICS_RejectsNonHTTPScheme(t *testing.T) {
	svc := newTestServiceWithStore(&fakeCalendarStore{}) //nolint:exhaustruct //unused

	_, err := svc.FetchICS(t.Context(), "file:///etc/passwd")
	require.ErrorContains(t, err, "unsupported scheme")
}

// A calendar bigger than the cap must be rejected rather than buffered whole —
// the api process is shared by every app.
func TestFetchICS_RejectsOversizedBody(t *testing.T) {
	chunk := strings.Repeat("x", 1<<20)
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			for range 6 {
				_, _ = w.Write([]byte(chunk))
			}
		},
	))
	defer srv.Close()

	svc := newTestServiceWithStore(&fakeCalendarStore{}) //nolint:exhaustruct //unused

	_, err := svc.FetchICS(t.Context(), srv.URL)
	require.ErrorIs(t, err, errCalendarTooLarge)
}

// Neither the error nor anything else we surface may echo the source URL back:
// a private Outlook/Google ICS link is a bearer credential and these strings
// end up in logs and Sentry.
func TestFetchICS_ErrorOmitsURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	))
	defer srv.Close()

	svc := newTestServiceWithStore(&fakeCalendarStore{}) //nolint:exhaustruct //unused

	secretURL := srv.URL + "/private/very-secret-token/basic.ics"
	_, err := svc.FetchICS(t.Context(), secretURL)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "very-secret-token")
}

func TestFetchICS_UnreachableHostErrorOmitsURL(t *testing.T) {
	svc := newTestServiceWithStore(&fakeCalendarStore{}) //nolint:exhaustruct //unused

	_, err := svc.FetchICS(
		t.Context(), "http://127.0.0.1:1/private/very-secret-token.ics",
	)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "very-secret-token")
}

// The production wiring must refuse to reach the container's own network.
func TestFetchICS_BlocksPrivateAddressesInProduction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(buildICS(
				vevent("uid-1", "x", "20240115T090000Z", "20240115T100000Z"),
			))
		},
	))
	defer srv.Close()

	svc := &CalendarService{
		logger: nil,
		repo:   &fakeCalendarStore{}, //nolint:exhaustruct //unused
		client: safedial.Client(calendarFetchTimeout, maxCalendarRedirects, false),
	}

	_, err := svc.FetchICS(t.Context(), srv.URL)
	require.ErrorContains(t, err, "fetching calendar from")
}

// ── malformed upstream calendars ─────────────────────────────────────────────

// An upstream VEVENT may omit any property, including UID and DTSTART.
// Dereferencing those unconditionally used to panic the request handler.
func TestExtractEvents_BareVEventDoesNotPanic(t *testing.T) {
	data := []byte(
		"BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//t//t//EN\r\n" +
			"BEGIN:VEVENT\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n",
	)

	svc := newTestServiceWithStore(&fakeCalendarStore{}) //nolint:exhaustruct //unused

	events, err := svc.ExtractEvents(t.Context(), data)
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Empty(t, events[0].UID)
}

func TestApplyFilter_BareVEventDoesNotPanic(t *testing.T) {
	data := []byte(
		"BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//t//t//EN\r\n" +
			"BEGIN:VEVENT\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n",
	)

	svc := newTestServiceWithStore(&fakeCalendarStore{}) //nolint:exhaustruct //unused

	//nolint:exhaustruct //only the holiday list matters here
	cfg := models.FilterConfig{HolidayUIDs: []string{"whatever"}}

	out, err := svc.ApplyFilter(t.Context(), data, cfg)
	require.NoError(t, err)
	assert.Contains(t, string(out), "BEGIN:VCALENDAR")
}

// A holiday event spanning centuries must not expand into millions of EXDATEs.
func TestApplyFilter_CapsHolidayExpansion(t *testing.T) {
	data := buildICS(
		vevent("holiday", "Away", "19700101T000000Z", "29991231T000000Z"),
		vevent(
			"weekly", "Standup", "20240115T090000Z", "20240115T093000Z",
			"RRULE:FREQ=WEEKLY",
		),
	)

	svc := newTestServiceWithStore(&fakeCalendarStore{}) //nolint:exhaustruct //unused

	//nolint:exhaustruct //only the holiday list matters here
	cfg := models.FilterConfig{HolidayUIDs: []string{"holiday"}}

	done := make(chan struct{})
	var out []byte
	var err error
	go func() {
		out, err = svc.ApplyFilter(t.Context(), data, cfg)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("ApplyFilter did not finish — holiday expansion is unbounded")
	}

	require.NoError(t, err)
	assert.LessOrEqual(t, strings.Count(string(out), ","), maxHolidayDays)
}

// ── SaveConfig ownership ─────────────────────────────────────────────────────

func TestSaveConfig_GeneratesTokenWhenEmpty(t *testing.T) {
	svc := newTestServiceWithStore(&fakeCalendarStore{}) //nolint:exhaustruct //unused

	//nolint:exhaustruct //UserID only
	token, err := svc.SaveConfig(t.Context(), models.FilterConfig{UserID: "owner"})
	require.NoError(t, err)
	assert.Len(t, token, 36, "expected a UUID")
}

func TestSaveConfig_RejectsUnknownToken(t *testing.T) {
	svc := newTestServiceWithStore(
		&fakeCalendarStore{found: false}, //nolint:exhaustruct //found only
	)

	//nolint:exhaustruct //Token/UserID only
	cfg := models.FilterConfig{Token: "guessable", UserID: "owner"}
	_, err := svc.SaveConfig(t.Context(), cfg)
	assert.ErrorIs(t, err, ErrConfigNotFound)
}

func TestSaveConfig_RejectsAnotherUsersToken(t *testing.T) {
	svc := newTestServiceWithStore(&fakeCalendarStore{
		found: true,
		//nolint:exhaustruct //UserID only
		cfg: models.FilterConfig{UserID: "someone-else"},
	})

	//nolint:exhaustruct //Token/UserID only
	cfg := models.FilterConfig{Token: "their-token", UserID: "attacker"}
	_, err := svc.SaveConfig(t.Context(), cfg)
	assert.ErrorIs(t, err, ErrConfigAccessDenied)
}

func TestSaveConfig_AcceptsOwnToken(t *testing.T) {
	svc := newTestServiceWithStore(&fakeCalendarStore{
		found: true,
		//nolint:exhaustruct //UserID only
		cfg: models.FilterConfig{UserID: "owner"},
	})

	//nolint:exhaustruct //Token/UserID only
	cfg := models.FilterConfig{Token: "own-token", UserID: "owner"}
	token, err := svc.SaveConfig(t.Context(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "own-token", token)
}
