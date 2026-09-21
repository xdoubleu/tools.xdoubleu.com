package slackwebhook_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/slackwebhook"
)

func TestSendNotConfigured(t *testing.T) {
	client := slackwebhook.New("")
	err := client.Send(t.Context(), "title", "message")
	assert.ErrorIs(t, err, slackwebhook.ErrNotConfigured)
}

func TestSendPostsExpectedRequestWithTitle(t *testing.T) {
	var gotMethod, gotURL, gotContentType string
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotURL = r.URL.String()
			gotContentType = r.Header.Get("Content-Type")
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
			w.WriteHeader(http.StatusOK)
		},
	))
	defer server.Close()

	client := slackwebhook.New(server.URL + "/services/hook")
	err := client.Send(t.Context(), "Epic complete", "Everything shipped.")
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/services/hook", gotURL)
	assert.Equal(t, "application/json", gotContentType)
	assert.Equal(t, "*Epic complete*\nEverything shipped.", gotBody["text"])
}

func TestSendPostsExpectedRequestWithoutTitle(t *testing.T) {
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
			w.WriteHeader(http.StatusOK)
		},
	))
	defer server.Close()

	client := slackwebhook.New(server.URL)
	err := client.Send(t.Context(), "", "Just the message.")
	require.NoError(t, err)

	assert.Equal(t, "Just the message.", gotBody["text"])
}

func TestSendReturnsErrorOnNonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("invalid_payload"))
		},
	))
	defer server.Close()

	client := slackwebhook.New(server.URL)
	err := client.Send(t.Context(), "title", "message")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}
