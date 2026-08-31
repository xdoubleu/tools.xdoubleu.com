package github_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/logging"
)

func TestDismissSecurityAlert_Dependabot_SendsPatchWithReason(t *testing.T) {
	var gotMethod, gotPath, gotBody string
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			buf := make([]byte, 512)
			n, _ := r.Body.Read(buf)
			gotBody = string(buf[:n])
			w.WriteHeader(http.StatusOK)
		}))
	defer cleanup()

	err := newClient().DismissSecurityAlert(
		context.Background(), github.SecurityAlertTypeDependabot, 83, "no_bandwidth",
	)
	require.NoError(t, err)
	assert.Equal(t, http.MethodPatch, gotMethod)
	assert.Equal(t, "/repos/"+testRepo+"/dependabot/alerts/83", gotPath)
	assert.Contains(t, gotBody, `"state":"dismissed"`)
	assert.Contains(t, gotBody, `"dismissed_reason":"no_bandwidth"`)
}

func TestDismissSecurityAlert_CodeScanning_SendsPatchWithReason(t *testing.T) {
	var gotPath, gotBody string
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			buf := make([]byte, 512)
			n, _ := r.Body.Read(buf)
			gotBody = string(buf[:n])
			w.WriteHeader(http.StatusOK)
		}))
	defer cleanup()

	err := newClient().DismissSecurityAlert(
		context.Background(), github.SecurityAlertTypeCodeScanning, 12, "won't fix",
	)
	require.NoError(t, err)
	assert.Equal(t, "/repos/"+testRepo+"/code-scanning/alerts/12", gotPath)
	assert.Contains(t, gotBody, `"state":"dismissed"`)
	assert.Contains(t, gotBody, `"dismissed_reason":"won't fix"`)
}

func TestDismissSecurityAlert_SecretScanning_SendsPatchWithResolution(t *testing.T) {
	var gotPath, gotBody string
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			buf := make([]byte, 512)
			n, _ := r.Body.Read(buf)
			gotBody = string(buf[:n])
			w.WriteHeader(http.StatusOK)
		}))
	defer cleanup()

	err := newClient().DismissSecurityAlert(
		context.Background(), github.SecurityAlertTypeSecretScanning, 7, "revoked",
	)
	require.NoError(t, err)
	assert.Equal(t, "/repos/"+testRepo+"/secret-scanning/alerts/7", gotPath)
	assert.Contains(t, gotBody, `"state":"resolved"`)
	assert.Contains(t, gotBody, `"resolution":"revoked"`)
}

func TestDismissSecurityAlert_InvalidReason(t *testing.T) {
	err := newClient().DismissSecurityAlert(
		context.Background(),
		github.SecurityAlertTypeDependabot, 1, "not_a_real_reason",
	)
	require.ErrorIs(t, err, github.ErrInvalidDismissReason)
}

func TestDismissSecurityAlert_ReasonValidForWrongType(t *testing.T) {
	// A dependabot-only reason isn't valid for code_scanning.
	err := newClient().DismissSecurityAlert(
		context.Background(), github.SecurityAlertTypeCodeScanning, 1, "no_bandwidth",
	)
	require.ErrorIs(t, err, github.ErrInvalidDismissReason)
}

func TestDismissSecurityAlert_SecretScanning_InvalidReason(t *testing.T) {
	// A dismissed_reason from the other two types isn't a valid resolution
	// for secret-scanning.
	err := newClient().DismissSecurityAlert(
		context.Background(), github.SecurityAlertTypeSecretScanning, 1, "no_bandwidth",
	)
	require.ErrorIs(t, err, github.ErrInvalidDismissReason)
}

func TestDismissSecurityAlert_UnknownAlertType(t *testing.T) {
	err := newClient().DismissSecurityAlert(
		context.Background(), github.SecurityAlertType("bogus"), 1, "whatever",
	)
	require.ErrorIs(t, err, github.ErrInvalidDismissReason)
}

func TestDismissSecurityAlert_NotConfigured_NoConnection(t *testing.T) {
	c := github.New(logging.NewNopLogger(), stubToken("unused"), configNotConnected())
	err := c.DismissSecurityAlert(
		context.Background(), github.SecurityAlertTypeDependabot, 1, "not_used",
	)
	require.ErrorIs(t, err, github.ErrNotConfigured)
}

func TestDismissSecurityAlert_NotConnected(t *testing.T) {
	c := github.New(
		logging.NewNopLogger(),
		stubNotConnected(),
		configWithRepo(testRepo),
	)
	err := c.DismissSecurityAlert(
		context.Background(), github.SecurityAlertTypeDependabot, 1, "not_used",
	)
	require.ErrorIs(t, err, github.ErrNotConfigured)
}

func TestDismissSecurityAlert_TokenFnGenericError(t *testing.T) {
	// A tokenFn error that isn't oauthconn.ErrNotConnected must propagate
	// as-is rather than being mapped to ErrNotConfigured.
	wantErr := errors.New("boom")
	c := github.New(
		logging.NewNopLogger(),
		func(context.Context) (string, error) { return "", wantErr },
		configWithRepo(testRepo),
	)
	err := c.DismissSecurityAlert(
		context.Background(), github.SecurityAlertTypeDependabot, 1, "not_used",
	)
	require.ErrorIs(t, err, wantErr)
}

func TestDismissSecurityAlert_InvalidRequestURL(t *testing.T) {
	// A repo containing a control character makes the built endpoint an
	// invalid URL, so http.NewRequestWithContext itself fails.
	c := github.New(
		logging.NewNopLogger(),
		stubToken("token"),
		configWithRepo("o/r\n"),
	)
	err := c.DismissSecurityAlert(
		context.Background(), github.SecurityAlertTypeDependabot, 1, "not_used",
	)
	require.Error(t, err)
}

func TestDismissSecurityAlert_TransportError(t *testing.T) {
	// Point at a server that's already been closed so the PATCH's
	// underlying http.Client.Do fails outright (connection refused),
	// without falling back to the real GitHub API.
	srv := httptest.NewServer(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	)
	closedURL := srv.URL
	srv.Close()
	github.SetBaseURL(closedURL)
	defer github.SetBaseURL(realBaseURL)

	err := newClient().DismissSecurityAlert(
		context.Background(), github.SecurityAlertTypeDependabot, 1, "not_used",
	)
	require.Error(t, err)
}

func TestDismissSecurityAlert_NonRetryableUpstreamError(t *testing.T) {
	// A 422 isn't retryable and isn't 2xx, so it must surface as an error
	// without exhausting all retry attempts.
	attempts := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			attempts++
			w.WriteHeader(http.StatusUnprocessableEntity)
		}))
	defer cleanup()

	err := newClient().DismissSecurityAlert(
		context.Background(), github.SecurityAlertTypeDependabot, 1, "not_used",
	)
	require.Error(t, err)
	assert.Equal(t, 1, attempts, "a non-retryable 4xx must not be retried")
}

func TestDismissSecurityAlert_ServerError_Retries(t *testing.T) {
	attempts := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			attempts++
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
	defer cleanup()

	err := newClient().DismissSecurityAlert(
		context.Background(), github.SecurityAlertTypeDependabot, 1, "not_used",
	)
	require.Error(t, err)
	assert.Equal(t, 4, attempts, "5xx must retry up to maxAttempts")
}

func TestDismissSecurityAlert_ClearsSecurityAlertsCache(t *testing.T) {
	listCalls := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/repos/" + testRepo + "/dependabot/alerts":
				listCalls++
				_, _ = w.Write([]byte(`[]`))
			case "/repos/" + testRepo + "/code-scanning/alerts",
				"/repos/" + testRepo + "/secret-scanning/alerts":
				_, _ = w.Write([]byte(`[]`))
			case "/repos/" + testRepo + "/dependabot/alerts/1":
				w.WriteHeader(http.StatusOK)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
	defer cleanup()

	c := newClient()
	_, err := c.ListSecurityAlerts(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, listCalls)

	// Still cached — a second List within cacheTTL doesn't re-fetch.
	_, err = c.ListSecurityAlerts(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, listCalls)

	err = c.DismissSecurityAlert(
		context.Background(), github.SecurityAlertTypeDependabot, 1, "not_used",
	)
	require.NoError(t, err)

	// Dismissing invalidated the cache, so this List re-fetches immediately.
	_, err = c.ListSecurityAlerts(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, listCalls)
}
