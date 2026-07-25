package mailer_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/mailer"
)

func TestSendNotConfigured(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
		from   string
		to     string
	}{
		{name: "no api key", apiKey: "", from: "a@b.com", to: "c@d.com"},
		{name: "no from", apiKey: "key", from: "", to: "c@d.com"},
		{name: "no to", apiKey: "key", from: "a@b.com", to: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := mailer.New(tt.apiKey, tt.from, tt.to)
			err := client.Send(t.Context(), "subject", "body")
			assert.ErrorIs(t, err, mailer.ErrNotConfigured)
		})
	}
}

func TestSendPostsExpectedRequest(t *testing.T) {
	var gotAuth string
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
			w.WriteHeader(http.StatusOK)
		},
	))
	defer server.Close()
	mailer.SetBaseURL(server.URL)
	defer mailer.SetBaseURL("https://api.resend.com")

	client := mailer.New("test-key", "from@example.com", "to@example.com")
	err := client.Send(t.Context(), "New Sentry issue", "details")
	require.NoError(t, err)

	assert.Equal(t, "Bearer test-key", gotAuth)
	assert.Equal(t, "from@example.com", gotBody["from"])
	assert.Equal(t, []any{"to@example.com"}, gotBody["to"])
	assert.Equal(t, "New Sentry issue", gotBody["subject"])
	assert.Equal(t, "details", gotBody["text"])
}

func TestSendReturnsErrorOnNonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("invalid api key"))
		},
	))
	defer server.Close()
	mailer.SetBaseURL(server.URL)
	defer mailer.SetBaseURL("https://api.resend.com")

	client := mailer.New("bad-key", "from@example.com", "to@example.com")
	err := client.Send(t.Context(), "subject", "body")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}
