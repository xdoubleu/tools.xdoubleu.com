package wstools

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// No hub on the context: falls back to sentry.CurrentHub() without panicking.
func TestAcceptWithHandshakeSpan_NoHubOnContext(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	conn, err := acceptWithHandshakeSpan(rec, req)

	require.Error(t, err)
	assert.Nil(t, conn)
}

func TestAcceptWithHandshakeSpan_WithHubOnContext(t *testing.T) {
	t.Parallel()

	hub := sentry.NewHub(nil, sentry.NewScope())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(sentry.SetHubOnContext(req.Context(), hub))
	rec := httptest.NewRecorder()

	conn, err := acceptWithHandshakeSpan(rec, req)

	require.Error(t, err)
	assert.Nil(t, conn)
}
