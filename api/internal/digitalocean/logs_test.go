package digitalocean_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
		`{"deployment":{"spec":{"services":[%s]}}}`, strings.Join(names, ","),
	)
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

// newLogsMux builds a fake DigitalOcean server: the deployment-detail
// endpoint reports the given components, and /logs returns historic URLs
// (served from the same test server, at /log-chunk/<key>) for every
// "component:type" key present in chunkKeys.
func newLogsMux(components []string, chunkKeys map[string][]string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(
		"/v2/apps/app-123/deployments/dep-1", deploymentDetailHandler(components...),
	)
	mux.HandleFunc(
		"/v2/apps/app-123/deployments/dep-1/logs",
		func(w http.ResponseWriter, r *http.Request) {
			key := r.URL.Query().Get("component_name") + ":" + r.URL.Query().Get("type")
			keys := chunkKeys[key]
			urls := make([]string, len(keys))
			for i, k := range keys {
				urls[i] = fmt.Sprintf("http://%s/log-chunk/%s", r.Host, k)
			}
			w.Header().Set("Content-Type", "application/json")
			body, _ := json.Marshal(map[string][]string{"historic_urls": urls})
			_, _ = w.Write(body)
		},
	)
	return mux
}

func TestDeploymentLogs_HappyPath(t *testing.T) {
	mux := newLogsMux(
		[]string{"api", "web"},
		map[string][]string{
			"api:BUILD":  {"api-build"},
			"web:DEPLOY": {"web-deploy"},
		},
	)
	mux.HandleFunc("/log-chunk/api-build", textHandler("building api\n"))
	mux.HandleFunc("/log-chunk/web-deploy", textHandler("deploying web\n"))
	cleanup := buildServer(mux)
	defer cleanup()

	logs, err := newClient().DeploymentLogs(context.Background(), "dep-1")
	require.NoError(t, err)
	require.Len(t, logs, 2)

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
}

func TestDeploymentLogs_SkipsMissingPhases(t *testing.T) {
	mux := newLogsMux([]string{"api"}, map[string][]string{})
	cleanup := buildServer(mux)
	defer cleanup()

	logs, err := newClient().DeploymentLogs(context.Background(), "dep-1")
	require.NoError(t, err)
	assert.Empty(t, logs)
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

	logs, err := newClient().DeploymentLogs(context.Background(), "dep-1")
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
		_, err := c.DeploymentLogs(context.Background(), "dep-1")
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

	_, err := newClient().DeploymentLogs(context.Background(), "dep-1")
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

	_, err := newClient().DeploymentLogs(context.Background(), "dep-1")
	require.Error(t, err)
	assert.Equal(t, 4, attempts)
}

func TestDeploymentLogs_DeploymentDetailError(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
	defer cleanup()

	_, err := newClient().DeploymentLogs(context.Background(), "dep-1")
	require.Error(t, err)
	require.NotErrorIs(t, err, digitalocean.ErrNotConfigured)
}
