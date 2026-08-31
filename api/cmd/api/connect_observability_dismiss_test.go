package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/logging"
)

func TestObservabilityDismissSecurityAlert_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}))
	t.Cleanup(srv.Close)
	github.SetBaseURL(srv.URL)
	t.Cleanup(func() { github.SetBaseURL("https://api.github.com") })
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		testConfigJSON(t, map[string]string{"repo": "o/r"}),
	)

	_, err := callDismissSecurityAlert(
		t, observabilityv1.SecurityAlertType_SECURITY_ALERT_TYPE_DEPENDABOT,
		83, "no_bandwidth",
	)
	require.NoError(t, err)
	assert.Equal(t, http.MethodPatch, gotMethod)
	assert.Equal(t, "/repos/o/r/dependabot/alerts/83", gotPath)
}

func TestObservabilityDismissSecurityAlert_CodeScanning(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}))
	t.Cleanup(srv.Close)
	github.SetBaseURL(srv.URL)
	t.Cleanup(func() { github.SetBaseURL("https://api.github.com") })
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		testConfigJSON(t, map[string]string{"repo": "o/r"}),
	)

	_, err := callDismissSecurityAlert(
		t, observabilityv1.SecurityAlertType_SECURITY_ALERT_TYPE_CODE_SCANNING,
		12, "won't fix",
	)
	require.NoError(t, err)
	assert.Equal(t, "/repos/o/r/code-scanning/alerts/12", gotPath)
}

func TestObservabilityDismissSecurityAlert_SecretScanning(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}))
	t.Cleanup(srv.Close)
	github.SetBaseURL(srv.URL)
	t.Cleanup(func() { github.SetBaseURL("https://api.github.com") })
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		testConfigJSON(t, map[string]string{"repo": "o/r"}),
	)

	_, err := callDismissSecurityAlert(
		t, observabilityv1.SecurityAlertType_SECURITY_ALERT_TYPE_SECRET_SCANNING,
		7, "revoked",
	)
	require.NoError(t, err)
	assert.Equal(t, "/repos/o/r/secret-scanning/alerts/7", gotPath)
}

func TestObservabilityDismissSecurityAlert_InvalidReason(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		testConfigJSON(t, map[string]string{"repo": "o/r"}),
	)

	_, err := callDismissSecurityAlert(
		t, observabilityv1.SecurityAlertType_SECURITY_ALERT_TYPE_DEPENDABOT,
		83, "not_a_real_reason",
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestObservabilityDismissSecurityAlert_NotConfigured(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		configNotConnected(),
	)

	_, err := callDismissSecurityAlert(
		t, observabilityv1.SecurityAlertType_SECURITY_ALERT_TYPE_DEPENDABOT,
		83, "not_used",
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestObservabilityDismissSecurityAlert_UpstreamError(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := jsonServer(t, http.StatusInternalServerError, ``)
	github.SetBaseURL(srv.URL)
	t.Cleanup(func() { github.SetBaseURL("https://api.github.com") })
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		testConfigJSON(t, map[string]string{"repo": "o/r"}),
	)

	_, err := callDismissSecurityAlert(
		t, observabilityv1.SecurityAlertType_SECURITY_ALERT_TYPE_DEPENDABOT,
		83, "not_used",
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestObservabilityDismissSecurityAlert_NonAdmin(t *testing.T) {
	demoteToUser(t)
	_, err := callDismissSecurityAlert(
		t, observabilityv1.SecurityAlertType_SECURITY_ALERT_TYPE_DEPENDABOT,
		83, "not_used",
	)
	requirePermissionDenied(t, err)
}

func callDismissSecurityAlert(
	t *testing.T,
	alertType observabilityv1.SecurityAlertType,
	alertNumber int64,
	reason string,
) (*connect.Response[observabilityv1.DismissSecurityAlertResponse], error) {
	t.Helper()
	req := connect.NewRequest(&observabilityv1.DismissSecurityAlertRequest{
		AlertType:   alertType,
		AlertNumber: alertNumber,
		Reason:      reason,
	})
	setCookieOnRequest(req, accessToken)
	return observabilityClient(t).DismissSecurityAlert(context.Background(), req)
}
