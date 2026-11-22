package resend_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/feeds/internal/resend"
)

func stub(t *testing.T, h http.HandlerFunc) *resend.Client {
	t.Helper()
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	resend.SetBaseURL(server.URL)
	t.Cleanup(func() { resend.SetBaseURL(resend.DefaultBaseURL) })
	return resend.New("key")
}

func TestHTML_ConnectionError(t *testing.T) {
	resend.SetBaseURL("http://127.0.0.1:1")
	t.Cleanup(func() { resend.SetBaseURL(resend.DefaultBaseURL) })

	_, err := resend.New("key").HTML(context.Background(), "email-1")
	require.Error(t, err)
}

func TestHTML_NonOKStatus(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("upstream error"))
	})

	_, err := c.HTML(context.Background(), "email-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upstream error")
}

func TestHTML_MalformedJSON(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	})

	_, err := c.HTML(context.Background(), "email-1")
	require.Error(t, err)
}

func TestHTML_SendsBearerKeyAndReturnsHTML(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer key", r.Header.Get("Authorization"))
		assert.Equal(t, "/emails/receiving/email-1", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"html": "<p>hi</p>", "text": "hi",
		})
	})

	html, err := c.HTML(context.Background(), "email-1")
	require.NoError(t, err)
	assert.Equal(t, "<p>hi</p>", html)
}

func TestHTML_TextFallback(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"text": "plain text body"})
	})

	html, err := c.HTML(context.Background(), "email-1")
	require.NoError(t, err)
	assert.Equal(t, "<pre>plain text body</pre>", html)
}

func TestHTML_NoHTMLOrText(t *testing.T) {
	c := stub(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{})
	})

	_, err := c.HTML(context.Background(), "email-1")
	require.Error(t, err)
}
