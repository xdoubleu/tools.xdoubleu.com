package github_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/github"
)

func TestListSecurityAlerts_AccessDenied_WrapsSentinel(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			attempts := 0
			cleanup := buildServer(http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) {
					attempts++
					w.WriteHeader(status)
					_, _ = w.Write([]byte(`{"message":"Resource not accessible"}`))
				}))
			defer cleanup()

			_, err := newClient().ListSecurityAlerts(context.Background())
			require.ErrorIs(t, err, github.ErrAccessDenied)
			assert.Contains(t, err.Error(), "Resource not accessible")
			assert.Equal(t, 1, attempts, "a denial is not retried")
		})
	}
}
