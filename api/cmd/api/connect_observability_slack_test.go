package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/slackwebhook"
)

// notify_slack has no Connect RPC/proto message of its own (issue #1628's
// scope is MCP-tool-only, see connect_observability_slack.go's doc
// comment), so these exercise the internal notifySlack method directly
// rather than going through a generated Connect client, unlike the other
// mutating observability tools' *_test.go files.

func TestObservabilityNotifySlack_Success(t *testing.T) {
	var gotMethod string
	var gotBody string

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			buf := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(buf)
			gotBody = string(buf)
			w.WriteHeader(http.StatusOK)
		}))
	t.Cleanup(srv.Close)

	//nolint:exhaustruct //only slackClient is used by notifySlack
	h := &obsConnectHandler{app: &Application{slackClient: slackwebhook.New(srv.URL)}}

	resp, err := h.notifySlack(t.Context(), "Epic complete", "All slices shipped.")
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Contains(t, gotBody, "Epic complete")
	assert.Contains(t, gotBody, "All slices shipped.")
}

func TestObservabilityNotifySlack_NotConfigured(t *testing.T) {
	//nolint:exhaustruct //only slackClient is used by notifySlack
	h := &obsConnectHandler{app: &Application{slackClient: slackwebhook.New("")}}

	_, err := h.notifySlack(t.Context(), "title", "message")
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestObservabilityNotifySlack_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
	t.Cleanup(srv.Close)

	//nolint:exhaustruct //only slackClient is used by notifySlack
	h := &obsConnectHandler{app: &Application{slackClient: slackwebhook.New(srv.URL)}}

	_, err := h.notifySlack(t.Context(), "title", "message")
	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}
