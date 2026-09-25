package sentryapi_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/sentryapi"
)

// Without org:read the picker can't list orgs.
func TestOAuthConfigScopes(t *testing.T) {
	cfg := sentryapi.OAuthConfig("id", "secret", "https://api.example.com")

	for _, scope := range []string{
		"org:read", "project:read", "event:read", "event:write",
	} {
		assert.Truef(t, slices.Contains(cfg.Scopes, scope),
			"missing scope %q", scope)
	}
}
