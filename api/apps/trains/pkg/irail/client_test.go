package irail_test

import (
	"compress/gzip"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/pkg/irail"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	f, err := os.Open("testdata/" + name + ".json.gz")
	require.NoError(t, err)
	defer f.Close()
	zr, err := gzip.NewReader(f)
	require.NoError(t, err)
	b, err := io.ReadAll(zr)
	require.NoError(t, err)
	return b
}

// newServer serves the recorded iRail responses.
func newServer(t *testing.T, hits *atomic.Int32) *irail.HTTPClient {
	t.Helper()
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			assert.NotEmpty(t, r.Header.Get("User-Agent"))
			switch r.URL.Path {
			case "/stations":
				_, _ = w.Write(fixture(t, "stations_"+r.URL.Query().Get("lang")))
			case "/connections":
				_, _ = w.Write(fixture(t, "connections"))
			case "/vehicle":
				if r.URL.Query().Get("id") != "BE.NMBS.IC530" {
					http.NotFound(w, r)
					return
				}
				_, _ = w.Write(fixture(t, "vehicle"))
			default:
				http.NotFound(w, r)
			}
		}),
	)
	t.Cleanup(srv.Close)
	return irail.New(slog.New(slog.DiscardHandler), srv.URL, "test-agent")
}

func TestStations(t *testing.T) {
	var hits atomic.Int32
	c := newServer(t, &hits)

	nl, err := c.Stations(context.Background(), "nl")
	require.NoError(t, err)
	fr, err := c.Stations(context.Background(), "fr")
	require.NoError(t, err)

	assert.Greater(t, len(nl), 600)
	byID := map[string]string{}
	for _, s := range fr {
		byID[s.ID] = s.Name
	}
	assert.Equal(t, "Bruxelles-Nord", byID["008812005"])
}

func TestConnections(t *testing.T) {
	var hits atomic.Int32
	c := newServer(t, &hits)

	conns, err := c.Connections(
		context.Background(), "008812005", "008841004",
		time.Date(2026, 10, 23, 8, 0, 0, 0, time.UTC), false,
	)
	require.NoError(t, err)
	require.Len(t, conns, 6)

	direct := conns[0]
	require.Len(t, direct.Legs, 1)
	assert.Equal(t, "008812005", direct.Legs[0].Board.StationID)
	assert.Equal(t, "008841004", direct.Legs[0].Alight.StationID)
	assert.Equal(
		t,
		"BE.NMBS."+direct.Legs[0].Type+direct.Legs[0].Number,
		direct.Legs[0].VehicleID,
	)

	var transfer irail.Connection
	for _, cn := range conns {
		if len(cn.Legs) == 2 {
			transfer = cn
			break
		}
	}
	require.Len(t, transfer.Legs, 2, "fixture holds a connection with one transfer")
	assert.Equal(t, transfer.Legs[0].Alight.StationID, transfer.Legs[1].Board.StationID)
	assert.NotEqual(t, transfer.Legs[0].VehicleID, transfer.Legs[1].VehicleID)
	assert.False(t, transfer.Legs[1].Board.Time.Before(transfer.Legs[0].Alight.Time))
}

func TestVehicleAndCache(t *testing.T) {
	var hits atomic.Int32
	c := newServer(t, &hits)
	day := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)

	v, err := c.Vehicle(context.Background(), "BE.NMBS.IC530", day)
	require.NoError(t, err)
	assert.Equal(t, "IC", v.Type)
	assert.Equal(t, "530", v.Num)
	require.NotEmpty(t, v.Calls)
	assert.False(t, v.Canceled())
	// The fixture was recorded mid-run, so some stop carries a delay.
	var delayed bool
	for _, call := range v.Calls {
		delayed = delayed || call.DepartureDelay > 0
	}
	assert.True(t, delayed)

	_, err = c.Vehicle(context.Background(), "BE.NMBS.IC530", day)
	require.NoError(t, err)
	assert.EqualValues(t, 1, hits.Load(), "second call is served from the cache")

	_, err = c.Vehicle(context.Background(), "BE.NMBS.IC1", day)
	require.ErrorIs(t, err, irail.ErrNotFound)
	assert.Equal(t, 2, c.Diagnostics().Requests)
}

func TestRateLimitRetriesOnceThenFails(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hits.Add(1)
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
		}),
	)
	t.Cleanup(srv.Close)
	c := irail.New(slog.New(slog.DiscardHandler), srv.URL, "ua")

	_, err := c.Stations(context.Background(), "en")
	require.ErrorIs(t, err, irail.ErrRateLimited)
	assert.EqualValues(t, 2, hits.Load())
	d := c.Diagnostics()
	assert.Equal(t, 2, d.RateLimited)
	assert.NotEmpty(t, d.Recent)
}

func TestVehicleCanceled(t *testing.T) {
	//nolint:exhaustruct //only the cancellation flags matter
	v := &irail.Vehicle{Calls: []irail.Call{
		{DepartureCanceled: true}, {ArrivalCanceled: true, DepartureCanceled: true},
	}}
	assert.True(t, v.Canceled())
	v.Calls = append(v.Calls, irail.Call{}) //nolint:exhaustruct //an uncancelled stop
	assert.False(t, v.Canceled())
}

func TestStationAndVehicleIDs(t *testing.T) {
	assert.Equal(t, "008812005", irail.StationUIC("BE.NMBS.008812005"))
	assert.Equal(t, "BE.NMBS.IC530", irail.VehicleID("IC 530"))
}
