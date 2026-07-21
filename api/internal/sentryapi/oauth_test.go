package sentryapi_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/sentryapi"
)

// TestOAuthConfigScopes guards that the org:read scope stays in the config —
// without it GET /api/0/organizations/ 403s and the admin picker can never
// list orgs.
func TestOAuthConfigScopes(t *testing.T) {
	cfg := sentryapi.OAuthConfig("id", "secret", "https://api.example.com")

	for _, scope := range []string{"org:read", "project:read", "event:read"} {
		assert.Truef(t, slices.Contains(cfg.Scopes, scope),
			"missing scope %q", scope)
	}
}
