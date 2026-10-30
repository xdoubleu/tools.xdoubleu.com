package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const wiringSnapshotPath = "testdata/wiring.golden"

// TestWiringSnapshot pins what the server wires: every route pattern with
// the status an anonymous and a signed-in non-admin probe get (its gate), and
// every /apps/mcp tool. UPDATE_WIRING_SNAPSHOT=1 rewrites the golden file.
func TestWiringSnapshot(t *testing.T) {
	got := wiringSnapshot(t)
	if os.Getenv("UPDATE_WIRING_SNAPSHOT") != "" {
		require.NoError(t, os.WriteFile(wiringSnapshotPath, []byte(got), 0o600))
	}

	want, err := os.ReadFile(wiringSnapshotPath)
	require.NoError(t, err)
	assert.Equal(t, string(want), got)
}

func wiringSnapshot(t *testing.T) string {
	t.Helper()

	mux := testApp.mux()
	userToken := freshTestUser(t)

	var b strings.Builder
	b.WriteString("# route pattern: anonymous status, non-admin user status\n")
	for _, pattern := range muxPatterns(t, mux) {
		fmt.Fprintf(&b, "%s: %d, %d\n", pattern,
			probeStatus(mux, pattern, ""), probeStatus(mux, pattern, userToken))
	}

	b.WriteString("# /apps/mcp tools\n")
	for _, name := range appsMCPToolNames(t) {
		b.WriteString(name + "\n")
	}
	return b.String()
}

// muxPatterns lists mux's registered patterns, sorted. ServeMux has no
// listing API, so it reads the index it keeps for conflict detection.
func muxPatterns(t *testing.T, mux *http.ServeMux) []string {
	t.Helper()

	index := reflect.ValueOf(mux).Elem().FieldByName("index")
	segments := index.FieldByName("segments")
	multis := index.FieldByName("multis")
	require.True(t, segments.IsValid() && multis.IsValid(),
		"http.ServeMux internals changed; update muxPatterns")

	seen := map[string]bool{}
	add := func(pats reflect.Value) {
		for i := range pats.Len() {
			seen[pats.Index(i).Elem().FieldByName("str").String()] = true
		}
	}
	add(multis)
	for iter := segments.MapRange(); iter.Next(); {
		add(iter.Value())
	}

	patterns := make([]string, 0, len(seen))
	for p := range seen {
		patterns = append(patterns, p)
	}
	slices.Sort(patterns)
	return patterns
}

var wildcardSegment = regexp.MustCompile(`\{[^}]*\}`)

// probeStatus requests pattern's path with no body, as token's user when
// set: a gate answers before the handler, and a Connect procedure without a
// content type is rejected before its handler runs.
func probeStatus(mux *http.ServeMux, pattern, token string) int {
	method, path, found := strings.Cut(pattern, " ")
	if !found {
		method, path = http.MethodGet, pattern
	}
	path = strings.ReplaceAll(path, "{$}", "")
	path = wildcardSegment.ReplaceAllString(path, "x")

	req := httptest.NewRequestWithContext(
		context.Background(), method, path, http.NoBody,
	)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "accessToken", Value: token})
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr.Code
}

// appsMCPToolNames lists the tools /apps/mcp registers, sorted.
func appsMCPToolNames(t *testing.T) []string {
	t.Helper()

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	_, err := testApp.newAppsMCPServer().Connect(t.Context(), serverTransport, nil)
	require.NoError(t, err)

	//nolint:exhaustruct // only Name/Version identify the test client
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(t.Context(), clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	res, err := session.ListTools(t.Context(), nil)
	require.NoError(t, err)

	names := make([]string, len(res.Tools))
	for i, tool := range res.Tools {
		names[i] = tool.Name
	}
	slices.Sort(names)
	return names
}
