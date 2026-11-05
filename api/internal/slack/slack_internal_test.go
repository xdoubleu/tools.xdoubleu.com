package slack

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEscapeCannotMakeALinkOrAMention(t *testing.T) {
	assert.Equal(t, "&lt;!channel&gt; &amp; &lt;http://x|y&gt;",
		Escape("<!channel> & <http://x|y>"))
	assert.Equal(t, "plain text", Escape("plain text"))
}

func TestOnlyAnHTTPSAddressMakesASender(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	assert.Nil(t, New("", logger))
	assert.Nil(t, New("http://hooks.slack.com/services/x", logger))
	assert.NotNil(t, New("https://hooks.slack.com/services/x", logger))
}

func TestNotifyPostsTheTextAsJSON(t *testing.T) {
	var mu sync.Mutex
	var got []string
	srv := httptest.NewTLSServer(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			got = append(got, r.Method+" "+r.Header.Get("Content-Type")+" "+string(body))
			mu.Unlock()
		}),
	)
	t.Cleanup(srv.Close)

	sender := newSender(
		srv.URL,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		srv.Client(),
	)
	require.NotNil(t, sender)
	sender.Notify(context.Background(), "Pull request opened")
	sender.Wait()

	assert.Equal(t, []string{`POST application/json {"text":"Pull request opened"}`}, got)
}

func TestAFailureIsLoggedWithoutTheWebhookAddress(t *testing.T) {
	srv := httptest.NewTLSServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}),
	)
	t.Cleanup(srv.Close)
	var logs bytes.Buffer
	sender := newSender(srv.URL, slog.New(slog.NewTextHandler(&logs, nil)), srv.Client())

	sender.Notify(context.Background(), "x")
	sender.Wait()
	assert.Contains(t, logs.String(), "Slack refused a notification")
	assert.Contains(t, logs.String(), "status=403")
	assert.NotContains(t, logs.String(), srv.URL, "the address is a secret")

	logs.Reset()
	closed := newSender(
		"https://127.0.0.1:1/hook",
		slog.New(slog.NewTextHandler(&logs, nil)),
		nil,
	)
	closed.Notify(context.Background(), "x")
	closed.Wait()
	assert.Contains(t, logs.String(), "could not post to Slack")
}
