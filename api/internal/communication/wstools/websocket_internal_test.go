package wstools

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAcceptWithHandshakeSpan_NoHubOnContext exercises the branch where the
// request context carries no Sentry hub (sentry.GetHubFromContext returns
// nil), falling back to sentry.CurrentHub() — the case a request that never
// went through sentryhttp's middleware (e.g. this bare httptest request)
// hits. It must not panic even though no client is configured, and still
// reports the accept failure.
func TestAcceptWithHandshakeSpan_NoHubOnContext(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	conn, err := acceptWithHandshakeSpan(rec, req)

	require.Error(t, err)
	assert.Nil(t, conn)
}

// TestAcceptWithHandshakeSpan_WithHubOnContext exercises the branch where
// the request context already carries a hub (sentry.GetHubFromContext
// returns non-nil) — the normal production path, since sentryhttp's
// middleware always sets one before this handler runs.
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
