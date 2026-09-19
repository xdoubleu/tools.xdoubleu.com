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

// fakeRecorder is a minimal stand-in for
// *repositories.AutomatedActionsRepository — exercising Fire's
// ordering/error-handling logic doesn't need a full Postgres-backed
// repository.
type fakeRecorder struct {
	openCalls   int32
	lastSource  string
	lastRoutine string
	openErr     error

	closeCalls    int32
	lastCloseID   int64
	lastOutcome   string
	lastErrorText string
	closeErr      error
}

func (f *fakeRecorder) Open(
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

func (f *fakeRecorder) Close(
	_ context.Context, id int64, outcome, _, errorText string,
) error {
	atomic.AddInt32(&f.closeCalls, 1)
	f.lastCloseID = id
	f.lastOutcome = outcome
	f.lastErrorText = errorText
	return f.closeErr
}

func newTestClient(baseURL, token string) (*routines.Client, *fakeRecorder) {
	//nolint:exhaustruct // zero-value fields are fine for a fake
	recorder := &fakeRecorder{}
	client := routines.NewClientForTest(baseURL, token, recorder)
	return client, recorder
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

	client, recorder := newTestClient(server.URL, "secret-token")

	err := client.Fire(t.Context(), "morning-repair", "the PR is failing")
	require.NoError(t, err)

	assert.Equal(t, int32(1), recorder.openCalls)
	assert.Equal(t, "api", recorder.lastSource)
	assert.Equal(t, "morning-repair", recorder.lastRoutine)

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
	client, recorder := newTestClient("http://example.invalid", "token")

	err := client.Fire(t.Context(), "", "text")
	require.Error(t, err)
	assert.Equal(t, int32(0), recorder.openCalls)
	assert.Equal(t, int32(0), recorder.closeCalls)
}

func TestFire_OpenFails_ReturnsErrorWithoutCallingWebhookOrClosing(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(
		func(_ http.ResponseWriter, _ *http.Request) {
			called = true
		},
	))
	defer server.Close()

	client, recorder := newTestClient(server.URL, "token")
	recorder.openErr = assert.AnError

	err := client.Fire(t.Context(), "morning-repair", "text")
	require.Error(t, err)
	assert.False(t, called)
	assert.Equal(t, int32(0), recorder.closeCalls)
}

// TestFire_WebhookReturnsErrorStatus_ClosesStrandedActionAsFailed exercises
// issue #1725's fix: a non-2xx response from the routine's fire webhook
// means the routine was never actually invoked, so Fire must close the row
// it just opened itself rather than leaving it stranded open forever.
func TestFire_WebhookReturnsErrorStatus_ClosesStrandedActionAsFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	))
	defer server.Close()

	client, recorder := newTestClient(server.URL, "token")

	err := client.Fire(t.Context(), "morning-repair", "text")
	require.Error(t, err)
	assert.Equal(t, int32(1), recorder.openCalls)
	assert.Equal(t, int32(1), recorder.closeCalls)
	assert.Equal(t, int64(1), recorder.lastCloseID)
	assert.Equal(t, "failed", recorder.lastOutcome)
	assert.Contains(t, recorder.lastErrorText, "unexpected status 500")
}

func TestFire_WebhookUnreachable_ClosesStrandedActionAsFailed(t *testing.T) {
	client, recorder := newTestClient("http://127.0.0.1:0", "token")

	err := client.Fire(t.Context(), "morning-repair", "text")
	require.Error(t, err)
	assert.Equal(t, int32(1), recorder.openCalls)
	assert.Equal(t, int32(1), recorder.closeCalls)
	assert.Equal(t, "failed", recorder.lastOutcome)
}

// TestFire_WebhookUnreachableAndCloseFails_ReturnsJoinedError covers the
// case where even the recovery Close call fails: both the original fire
// failure and the close failure must surface, rather than one silently
// swallowing the other, since that combination is exactly the case where
// the row is left open (the case AutomatedActionStalled exists to catch).
func TestFire_WebhookUnreachableAndCloseFails_ReturnsJoinedError(t *testing.T) {
	client, recorder := newTestClient("http://127.0.0.1:0", "token")
	recorder.closeErr = assert.AnError

	err := client.Fire(t.Context(), "morning-repair", "text")
	require.Error(t, err)
	assert.Equal(t, int32(1), recorder.closeCalls)
	require.ErrorIs(t, err, assert.AnError)
}

func TestNewClient_DefaultsHTTPClient(t *testing.T) {
	client := routines.NewClient("https://example.com", "token", nil)
	assert.NotNil(t, client.HTTPClient)
}

func TestFire_MalformedBaseURL_ClosesStrandedActionAsFailed(t *testing.T) {
	client, recorder := newTestClient("://not-a-valid-url", "token")

	err := client.Fire(t.Context(), "morning-repair", "text")
	require.Error(t, err)
	assert.Equal(t, int32(1), recorder.openCalls)
	assert.Equal(t, int32(1), recorder.closeCalls)
	assert.Equal(t, "failed", recorder.lastOutcome)
}

func TestFire_NilHTTPClient_FallsBackToDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	))
	defer server.Close()

	client, recorder := newTestClient(server.URL, "token")
	client.HTTPClient = nil

	err := client.Fire(t.Context(), "morning-repair", "text")
	require.NoError(t, err)
	assert.Equal(t, int32(1), recorder.openCalls)
}
