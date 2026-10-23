// Package mocks holds iRail test doubles for the trains app.
package mocks

import (
	"compress/gzip"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"tools.xdoubleu.com/apps/trains/pkg/irail"
)

// StubClient is an irail.Client whose answers each test sets; an unset func
// returns nothing.
type StubClient struct {
	StationsFn    func(lang string) ([]irail.Station, error)
	ConnectionsFn func(from, to string) ([]irail.Connection, error)
	VehicleFn     func(vehicleID string) (*irail.Vehicle, error)
	// Calls counts Vehicle lookups.
	Calls int
}

var _ irail.Client = (*StubClient)(nil)

func (s *StubClient) Stations(_ context.Context, lang string) ([]irail.Station, error) {
	if s.StationsFn == nil {
		return nil, nil
	}
	return s.StationsFn(lang)
}

func (s *StubClient) Connections(
	_ context.Context, from, to string, _ time.Time, _ bool,
) ([]irail.Connection, error) {
	if s.ConnectionsFn == nil {
		return nil, nil
	}
	return s.ConnectionsFn(from, to)
}

func (s *StubClient) Vehicle(
	_ context.Context, vehicleID string, _ time.Time,
) (*irail.Vehicle, error) {
	s.Calls++
	if s.VehicleFn == nil {
		return nil, irail.ErrNotFound
	}
	return s.VehicleFn(vehicleID)
}

func (s *StubClient) RawVehicle(
	_ context.Context, _ string, _ time.Time,
) ([]byte, error) {
	return []byte(`{}`), nil
}

func (s *StubClient) Diagnostics() irail.Diagnostics {
	//nolint:exhaustruct //a stub has no request history
	return irail.Diagnostics{}
}

// NewFixtureClient returns the real client pointed at a local server that
// replays the responses recorded in pkg/irail/testdata: stations in each
// language, Brussels-North to Liège-Guillemins connections, and train IC 530.
func NewFixtureClient(t testing.TB) *irail.HTTPClient {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "pkg", "irail", "testdata")

	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			name := ""
			switch r.URL.Path {
			case "/stations":
				name = "stations_" + r.URL.Query().Get("lang")
			case "/connections":
				name = "connections"
			case "/vehicle":
				if r.URL.Query().Get("id") == "BE.NMBS.IC530" {
					name = "vehicle"
				}
			}
			if name == "" {
				http.NotFound(w, r)
				return
			}
			body, err := readGzip(filepath.Join(dir, name+".json.gz"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_, _ = w.Write(body) //nolint:gosec //test server replaying recorded JSON
		}),
	)
	t.Cleanup(srv.Close)
	return irail.New(slog.New(slog.DiscardHandler), srv.URL, "trains-test")
}

func readGzip(path string) ([]byte, error) {
	f, err := os.Open(path) //nolint:gosec //path is built from fixed fixture names
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(zr)
}
