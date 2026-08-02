package digitalocean_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/internal/digitalocean"
)

func textHandler(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}
}

func deploymentDetailHandler(components ...string) http.HandlerFunc {
	names := make([]string, len(components))
	for i, c := range components {
		names[i] = fmt.Sprintf(`{"name":%q}`, c)
	}
	body := fmt.Sprintf(
		`{"deployment":{"services":[%s]}}`, strings.Join(names, ","),
	)
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

// doServer is a fake DigitalOcean API covering both log scopes: the
// deployment-scoped /logs endpoint and the app-scoped one that resolves to
// whichever deployment is serving traffic, plus the two detail endpoints that
// name their service components.
//
// chunks/live are keyed by "component:TYPE"; a chunks value lists keys served
// as historic URLs at /log-chunk/<key>, a live value is a path on the same
// test server that speaks websocket.
type doServer struct {
	components  []string
	chunks      map[string][]string
	live        map[string]string
	activeID    string
	activeComps []string
	appChunks   map[string][]string
	appLive     map[string]string
	// appDetailStatus, when set, makes the app-detail endpoint fail with it.
	appDetailStatus int
	// errStatus/errBody, keyed like chunks/live, make a specific
	// component:TYPE pair fail the /logs request with that status/body
	// instead of returning a normal payload.
	errStatus map[string]int
	errBody   map[string]string

	mu      sync.Mutex
	queries []url.Values
}

func (s *doServer) mux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(
		"/v2/apps/app-123/deployments/dep-1",
		deploymentDetailHandler(s.components...),
	)
	mux.HandleFunc(
		"/v2/apps/app-123/deployments/dep-1/logs",
		s.logsHandler(s.chunks, s.live),
	)
	mux.HandleFunc("/v2/apps/app-123", s.appDetailHandler())
	mux.HandleFunc("/v2/apps/app-123/logs", s.logsHandler(s.appChunks, s.appLive))
	return mux
}

func (s *doServer) logsHandler(
	chunks map[string][]string, live map[string]string,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		s.mu.Lock()
		s.queries = append(s.queries, query)
		s.mu.Unlock()

		key := query.Get("component_name") + ":" + query.Get("type")

		if status, ok := s.errStatus[key]; ok {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(s.errBody[key]))
			return
		}

		urls := make([]string, 0, len(chunks[key]))
		for _, k := range chunks[key] {
			urls = append(urls, fmt.Sprintf("http://%s/log-chunk/%s", r.Host, k))
		}

		payload := map[string]any{"historic_urls": urls}
		if path, ok := live[key]; ok {
			payload["live_url"] = fmt.Sprintf("http://%s%s", r.Host, path)
		}

		w.Header().Set("Content-Type", "application/json")
		body, _ := json.Marshal(payload)
		_, _ = w.Write(body)
	}
}

func (s *doServer) appDetailHandler() http.HandlerFunc {
	names := make([]string, len(s.activeComps))
	for i, c := range s.activeComps {
		names[i] = fmt.Sprintf(`{"name":%q}`, c)
	}
	body := fmt.Sprintf(
		`{"app":{"active_deployment":{"id":%q,"services":[%s]}}}`,
		s.activeID, strings.Join(names, ","),
	)
	return func(w http.ResponseWriter, _ *http.Request) {
		if s.appDetailStatus != 0 {
			w.WriteHeader(s.appDetailStatus)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

// recordedQueries returns every /logs query the server saw.
func (s *doServer) recordedQueries() []url.Values {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.queries)
}

// newLogsMux builds a fake DigitalOcean server whose active deployment is the
// one being asked about — the ordinary case, where no app-scoped runtime
// fetch is needed.
func newLogsMux(components []string, chunkKeys map[string][]string) *http.ServeMux {
	//nolint:exhaustruct // the active-deployment fields are exercised separately
	return (&doServer{
		components: components,
		chunks:     chunkKeys,
		activeID:   "dep-1",
	}).mux()
}

func TestDeploymentLogs_HappyPath(t *testing.T) {
	mux := newLogsMux(
		[]string{"api", "web"},
		map[string][]string{
			"api:BUILD":  {"api-build"},
			"web:DEPLOY": {"web-deploy"},
			"api:RUN":    {"api-run"},
		},
	)
	mux.HandleFunc("/log-chunk/api-build", textHandler("building api\n"))
	mux.HandleFunc("/log-chunk/web-deploy", textHandler("deploying web\n"))
	mux.HandleFunc("/log-chunk/api-run", textHandler("running api\n"))
	cleanup := buildServer(mux)
	defer cleanup()

	logs, err := newClient().DeploymentLogs(context.Background(), "dep-1", 0)
	require.NoError(t, err)
	require.Len(t, logs, 3)

	byKey := map[string]digitalocean.ComponentLog{}
	for _, l := range logs {
		byKey[l.Component+":"+string(l.Type)] = l
	}

	apiBuild, ok := byKey["api:BUILD"]
	require.True(t, ok)
	assert.Equal(t, "building api\n", apiBuild.Content)
	assert.False(t, apiBuild.Truncated)

	webDeploy, ok := byKey["web:DEPLOY"]
	require.True(t, ok)
	assert.Equal(t, "deploying web\n", webDeploy.Content)

	apiRun, ok := byKey["api:RUN"]
	require.True(t, ok)
	assert.Equal(t, "running api\n", apiRun.Content)
}

func TestDeploymentLogsStream_HappyPath(t *testing.T) {
	mux := newLogsMux(
		[]string{"api", "web"},
		map[string][]string{
			"api:BUILD":  {"api-build"},
			"web:DEPLOY": {"web-deploy"},
			"api:RUN":    {"api-run"},
		},
	)
	mux.HandleFunc("/log-chunk/api-build", textHandler("building api\n"))
	mux.HandleFunc("/log-chunk/web-deploy", textHandler("deploying web\n"))
	mux.HandleFunc("/log-chunk/api-run", textHandler("running api\n"))
	cleanup := buildServer(mux)
	defer cleanup()

	var mu sync.Mutex
	byKey := map[string]digitalocean.ComponentLog{}
	err := newClient().DeploymentLogsStream(
		context.Background(), "dep-1", 0,
		func(log digitalocean.ComponentLog) error {
			mu.Lock()
			defer mu.Unlock()
			byKey[log.Component+":"+string(log.Type)] = log
			return nil
		},
	)
	require.NoError(t, err)
	require.Len(t, byKey, 3)

	apiBuild, ok := byKey["api:BUILD"]
	require.True(t, ok)
	assert.Equal(t, "building api\n", apiBuild.Content)
	assert.False(t, apiBuild.Truncated)

	webDeploy, ok := byKey["web:DEPLOY"]
	require.True(t, ok)
	assert.Equal(t, "deploying web\n", webDeploy.Content)

	apiRun, ok := byKey["api:RUN"]
	require.True(t, ok)
	assert.Equal(t, "running api\n", apiRun.Content)
}

func TestDeploymentLogsStream_YieldErrorAbortsAndIsReturned(t *testing.T) {
	mux := newLogsMux(
		[]string{"api"},
		map[string][]string{"api:BUILD": {"api-build"}},
	)
	mux.HandleFunc("/log-chunk/api-build", textHandler("building api\n"))
	cleanup := buildServer(mux)
	defer cleanup()

	yieldErr := errors.New("client gone")
	err := newClient().DeploymentLogsStream(
		context.Background(), "dep-1", 0,
		func(digitalocean.ComponentLog) error {
			return yieldErr
		},
	)
	require.ErrorIs(t, err, yieldErr)
}

func TestDeploymentLogsStream_NotConfigured(t *testing.T) {
	called := false
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
	defer cleanup()

	client := digitalocean.New(
		logging.NewNopLogger(),
		stubNotConnected(),
		configWithAppID("app-123"),
	)

	yielded := false
	err := client.DeploymentLogsStream(
		context.Background(), "dep-1", 0,
		func(digitalocean.ComponentLog) error {
			yielded = true
			return nil
		},
	)
	require.ErrorIs(t, err, digitalocean.ErrNotConfigured)
	assert.False(t, yielded)
	assert.False(t, called)
}

func TestDeploymentLogs_SkipsMissingPhases(t *testing.T) {
	mux := newLogsMux([]string{"api"}, map[string][]string{})
	cleanup := buildServer(mux)
	defer cleanup()

	logs, err := newClient().DeploymentLogs(context.Background(), "dep-1", 0)
	require.NoError(t, err)
	assert.Empty(t, logs)
}

func TestDeploymentLogs_SkipsBuildLogTaskSkipped(t *testing.T) {
	//nolint:exhaustruct // the active-deployment fields are exercised separately
	server := &doServer{
		components: []string{"api"},
		chunks:     map[string][]string{"api:DEPLOY": {"api-deploy"}},
		activeID:   "dep-1",
		errStatus:  map[string]int{"api:BUILD": http.StatusBadRequest},
		errBody: map[string]string{
			"api:BUILD": `{"id":"bad_request","message":"cannot get build ` +
				`logs for component app with log task status skipped"}`,
		},
	}
	mux := server.mux()
	mux.HandleFunc("/log-chunk/api-deploy", textHandler("deploying api\n"))
	cleanup := buildServer(mux)
	defer cleanup()

	logs, err := newClient().DeploymentLogs(context.Background(), "dep-1", 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, digitalocean.LogTypeDeploy, logs[0].Type)
}

func TestDeploymentLogs_ConcatenatesAndTruncatesChunks(t *testing.T) {
	mux := newLogsMux(
		[]string{"api"},
		map[string][]string{"api:BUILD": {"chunk-1", "chunk-2"}},
	)
	mux.HandleFunc("/log-chunk/chunk-1", textHandler(strings.Repeat("a", 200*1024)))
	mux.HandleFunc("/log-chunk/chunk-2", textHandler("should be truncated away"))
	cleanup := buildServer(mux)
	defer cleanup()

	logs, err := newClient().DeploymentLogs(context.Background(), "dep-1", 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.True(t, logs[0].Truncated)
	assert.Len(t, logs[0].Content, 200*1024)
}

func TestDeploymentLogs_NotConfigured(t *testing.T) {
	called := false
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
	defer cleanup()

	cases := []digitalocean.Client{
		digitalocean.New(
			logging.NewNopLogger(),
			stubNotConnected(),
			configWithAppID("app-123"),
		),
		digitalocean.New(
			logging.NewNopLogger(),
			stubToken("token"),
			configWithAppID(""),
		),
		digitalocean.New(
			logging.NewNopLogger(),
			stubToken("token"),
			configNotConnected(),
		),
	}
	for _, c := range cases {
		_, err := c.DeploymentLogs(context.Background(), "dep-1", 0)
		require.ErrorIs(t, err, digitalocean.ErrNotConfigured)
	}
	assert.False(t, called, "must not hit the API when unconfigured")
}

func TestDeploymentLogs_LogChunkNonRetryableError(t *testing.T) {
	mux := newLogsMux(
		[]string{"api"},
		map[string][]string{"api:BUILD": {"missing-chunk"}},
	)
	mux.HandleFunc(
		"/log-chunk/missing-chunk",
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
	)
	cleanup := buildServer(mux)
	defer cleanup()

	_, err := newClient().DeploymentLogs(context.Background(), "dep-1", 0)
	require.Error(t, err)
	require.NotErrorIs(t, err, digitalocean.ErrNotConfigured)
}

func TestDeploymentLogs_LogChunkServerError_Retries(t *testing.T) {
	digitalocean.SetBackoffBase(time.Millisecond)
	attempts := 0
	mux := newLogsMux(
		[]string{"api"},
		map[string][]string{"api:BUILD": {"flaky-chunk"}},
	)
	mux.HandleFunc(
		"/log-chunk/flaky-chunk",
		func(w http.ResponseWriter, _ *http.Request) {
			attempts++
			w.WriteHeader(http.StatusBadGateway)
		},
	)
	cleanup := buildServer(mux)
	defer cleanup()

	_, err := newClient().DeploymentLogs(context.Background(), "dep-1", 0)
	require.Error(t, err)
	assert.Equal(t, 4, attempts)
}

func TestDeploymentLogs_DeploymentDetailError(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
	defer cleanup()

	_, err := newClient().DeploymentLogs(context.Background(), "dep-1", 0)
	require.Error(t, err)
	require.NotErrorIs(t, err, digitalocean.ErrNotConfigured)
	assert.Contains(t, err.Error(), "service components")
}

// A failure fetching the /logs endpoint itself (as opposed to a historic-URL
// chunk) must name the component/type it was fetching for, since that
// endpoint is called once per component/type pair and the bare upstream
// error alone doesn't say which one failed. Only the BUILD pair is made to
// fail here (the others succeed) so the assertion stays deterministic even
// though component/type pairs are now fetched concurrently.
func TestDeploymentLogs_ComponentLogEndpointError(t *testing.T) {
	//nolint:exhaustruct // only the failing BUILD pair matters here
	server := &doServer{
		components: []string{"api"},
		activeID:   "dep-1",
		errStatus:  map[string]int{"api:BUILD": http.StatusUnauthorized},
	}
	cleanup := buildServer(server.mux())
	defer cleanup()

	_, err := newClient().DeploymentLogs(context.Background(), "dep-1", 0)
	require.Error(t, err)
	require.NotErrorIs(t, err, digitalocean.ErrNotConfigured)
	assert.Contains(t, err.Error(), "component api type BUILD")
}

// tail_lines has to be on the query or DigitalOcean replays no backlog at all
// over the live socket, and follow has to be off or the socket never closes.
func TestDeploymentLogs_SendsFollowAndTailLines(t *testing.T) {
	cases := []struct {
		name     string
		tail     int
		expected string
	}{
		{name: "default", tail: 0, expected: "1000"},
		{name: "explicit", tail: 50, expected: "50"},
		{name: "clamped", tail: 999999, expected: "10000"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			//nolint:exhaustruct // only the deployment scope is exercised here
			server := &doServer{components: []string{"api"}, activeID: "dep-1"}
			cleanup := buildServer(server.mux())
			defer cleanup()

			_, err := newClient().DeploymentLogs(context.Background(), "dep-1", tc.tail)
			require.NoError(t, err)

			queries := server.recordedQueries()
			require.NotEmpty(t, queries)
			for _, q := range queries {
				assert.Equal(t, "false", q.Get("follow"))
				assert.Equal(t, tc.expected, q.Get("tail_lines"))
			}
		})
	}
}

// When the deployment being asked about is not the one serving traffic — a
// failed deploy on top of a still-running previous one — the runtime logs
// have to come from the active deployment, tagged as such.
func TestDeploymentLogs_AppendsActiveDeploymentRuntimeLogs(t *testing.T) {
	server := &doServer{
		components:      []string{"api"},
		chunks:          map[string][]string{"api:BUILD": {"failed-build"}},
		live:            nil,
		activeID:        "dep-0",
		activeComps:     []string{"api"},
		appChunks:       map[string][]string{"api:RUN": {"active-run"}},
		appLive:         nil,
		appDetailStatus: 0,
		errStatus:       nil,
		errBody:         nil,
		mu:              sync.Mutex{},
		queries:         nil,
	}
	mux := server.mux()
	mux.HandleFunc("/log-chunk/failed-build", textHandler("build failed\n"))
	mux.HandleFunc("/log-chunk/active-run", textHandler("still serving\n"))
	cleanup := buildServer(mux)
	defer cleanup()

	logs, err := newClient().DeploymentLogs(context.Background(), "dep-1", 0)
	require.NoError(t, err)
	require.Len(t, logs, 2)

	assert.Equal(t, digitalocean.LogTypeBuild, logs[0].Type)
	assert.Equal(t, "dep-1", logs[0].DeploymentID)
	assert.Equal(t, "build failed\n", logs[0].Content)

	assert.Equal(t, digitalocean.LogTypeRun, logs[1].Type)
	assert.Equal(t, "dep-0", logs[1].DeploymentID)
	assert.Equal(t, "still serving\n", logs[1].Content)
}

// The app-detail lookup is an addition, so losing it must not cost the caller
// the logs already collected for the requested deployment.
func TestDeploymentLogs_ActiveDeploymentErrorDegrades(t *testing.T) {
	//nolint:exhaustruct // only the failing app-detail path matters here
	server := &doServer{
		components:      []string{"api"},
		chunks:          map[string][]string{"api:BUILD": {"b"}},
		appDetailStatus: http.StatusInternalServerError,
	}
	mux := server.mux()
	mux.HandleFunc("/log-chunk/b", textHandler("build output\n"))
	digitalocean.SetBackoffBase(time.Millisecond)
	cleanup := buildServer(mux)
	defer cleanup()

	logs, err := newClient().DeploymentLogs(context.Background(), "dep-1", 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "build output\n", logs[0].Content)
}
