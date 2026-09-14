package routines_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/routines"
)

// fakeOpener is a minimal stand-in for *repositories.AutomatedActionsRepository
// — Client only ever calls Open on it, so a full Postgres-backed repository
// isn't needed to exercise Fire's ordering/error-handling logic.
type fakeOpener struct {
	openCalls   int32
	lastSource  string
	lastRoutine string
	openErr     error
}

func (f *fakeOpener) Open(
	_ context.Context, triggerSource, routineName string,
) (int64, error) {
	atomic.AddInt32(&f.openCalls, 1)
	f.lastSource = triggerSource
	f.lastRoutine = routineName
	if f.openErr != nil {
		return 0, f.openErr
	}
	return 1, nil
}

func newTestClient(baseURL, token string) (*routines.Client, *fakeOpener) {
	opener := &fakeOpener{} //nolint:exhaustruct // zero-value fields are fine for a fake
	client := routines.NewClientForTest(baseURL, token, opener)
	return client, opener
}

func TestFire_Success_OpensActionThenPostsExpectedRequest(t *testing.T) {
	var gotAuth, gotPath, gotMethod string
	var gotBody []byte

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			gotPath = r.URL.Path
			gotMethod = r.Method
			var err error
			gotBody, err = io.ReadAll(r.Body)
			require.NoError(t, err)
			w.WriteHeader(http.StatusNoContent)
		},
	))
	defer server.Close()

	client, opener := newTestClient(server.URL, "secret-token")

	err := client.Fire(t.Context(), "morning-repair", "the PR is failing")
	require.NoError(t, err)

	assert.Equal(t, int32(1), opener.openCalls)
	assert.Equal(t, "api", opener.lastSource)
	assert.Equal(t, "morning-repair", opener.lastRoutine)

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/morning-repair/fire", gotPath)
	assert.Equal(t, "Bearer secret-token", gotAuth)

	var payload struct {
		Text string `json:"text"`
	}
	require.NoError(t, json.Unmarshal(gotBody, &payload))
	assert.Equal(t, "the PR is failing", payload.Text)
}

func TestFire_EmptyRoutineName_ReturnsErrorWithoutOpeningAction(t *testing.T) {
	client, opener := newTestClient("http://example.invalid", "token")

	err := client.Fire(t.Context(), "", "text")
	require.Error(t, err)
	assert.Equal(t, int32(0), opener.openCalls)
}

func TestFire_OpenFails_ReturnsErrorWithoutCallingWebhook(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(
		func(_ http.ResponseWriter, _ *http.Request) {
			called = true
		},
	))
	defer server.Close()

	client, opener := newTestClient(server.URL, "token")
	opener.openErr = assert.AnError

	err := client.Fire(t.Context(), "morning-repair", "text")
	require.Error(t, err)
	assert.False(t, called)
}

func TestFire_WebhookReturnsErrorStatus_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	))
	defer server.Close()

	client, opener := newTestClient(server.URL, "token")

	err := client.Fire(t.Context(), "morning-repair", "text")
	require.Error(t, err)
	assert.Equal(t, int32(1), opener.openCalls)
}

func TestFire_WebhookUnreachable_ReturnsError(t *testing.T) {
	client, opener := newTestClient("http://127.0.0.1:0", "token")

	err := client.Fire(t.Context(), "morning-repair", "text")
	require.Error(t, err)
	assert.Equal(t, int32(1), opener.openCalls)
}

func TestNewClient_DefaultsHTTPClient(t *testing.T) {
	client := routines.NewClient("https://example.com", "token", nil)
	assert.NotNil(t, client.HTTPClient)
}

func TestFire_MalformedBaseURL_ReturnsErrorAfterOpeningAction(t *testing.T) {
	client, opener := newTestClient("://not-a-valid-url", "token")

	err := client.Fire(t.Context(), "morning-repair", "text")
	require.Error(t, err)
	assert.Equal(t, int32(1), opener.openCalls)
}

func TestFire_NilHTTPClient_FallsBackToDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	))
	defer server.Close()

	client, opener := newTestClient(server.URL, "token")
	client.HTTPClient = nil

	err := client.Fire(t.Context(), "morning-repair", "text")
	require.NoError(t, err)
	assert.Equal(t, int32(1), opener.openCalls)
}
