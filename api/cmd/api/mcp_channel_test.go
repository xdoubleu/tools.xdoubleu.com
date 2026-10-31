package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
)

// TestMCPBearerRouteRecordsMCPChannel: a bearer request carries the same
// user as a cookie session, so only the channel tells them apart.
func TestMCPBearerRouteRecordsMCPChannel(t *testing.T) {
	var channel *string
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		channel = contexttools.GetValue[string](
			r.Context(), constants.AuthChannelContextKey,
		)
	})
	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, "/", nil,
	)
	req.Header.Set("Authorization", "Bearer "+freshTestUser(t))

	testApp.mcpBearerRoute("", inner).ServeHTTP(httptest.NewRecorder(), req)

	require.NotNil(t, channel)
	assert.Equal(t, constants.ChannelMCP, *channel)
}
