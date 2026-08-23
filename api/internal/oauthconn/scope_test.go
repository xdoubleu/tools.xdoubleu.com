package oauthconn_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/oauthconn"
)

func TestScopesAreStale(t *testing.T) {
	githubRequired := []string{"repo", "security_events"}

	tests := map[string]struct {
		requested string
		granted   string
		required  []string
		want      bool
	}{
		// The bug this function exists for: GitHub normalizes the granted
		// scope down to `repo`, which already subsumes `security_events`, so
		// judging coverage by the echo marked a fully-authorized connection
		// stale on every single reconnect.
		"github's normalized echo omits a subsumed scope": {
			requested: "repo security_events",
			granted:   "repo",
			required:  githubRequired,
			want:      false,
		},
		"authorized before a scope was added": {
			requested: "repo",
			granted:   "repo",
			required:  githubRequired,
			want:      true,
		},
		"requested covers required exactly": {
			requested: "repo security_events",
			granted:   "repo,security_events",
			required:  githubRequired,
			want:      false,
		},
		// Rows predating requested_scope fall back to the granted check, so
		// they keep their previous behavior until the next reconnect.
		"legacy row with a covering granted scope": {
			requested: "",
			granted:   "org:read event:write",
			required:  []string{"org:read", "event:write"},
			want:      false,
		},
		"legacy row missing a required scope": {
			requested: "",
			granted:   "org:read",
			required:  []string{"org:read", "event:write"},
			want:      true,
		},
		"legacy row with no recorded scope at all": {
			requested: "",
			granted:   "",
			required:  githubRequired,
			want:      false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			//nolint:exhaustruct // only the scope fields matter here
			conn := &models.OAuthConnection{
				RequestedScope: tc.requested,
				GrantedScope:   tc.granted,
			}
			assert.Equal(t, tc.want, oauthconn.ScopesAreStale(conn, tc.required))
		})
	}
}

func TestScopesAreStaleNilConnection(t *testing.T) {
	assert.False(t, oauthconn.ScopesAreStale(nil, []string{"repo"}))
}

func TestHasScopes(t *testing.T) {
	tests := map[string]struct {
		granted  string
		required []string
		want     bool
	}{
		"empty granted is treated as unknown, not missing": {
			granted:  "",
			required: []string{"event:write"},
			want:     true,
		},
		"granted covers required": {
			granted:  "org:read project:read event:write",
			required: []string{"org:read", "event:write"},
			want:     true,
		},
		"granted missing a required scope": {
			granted:  "org:read project:read event:read",
			required: []string{"org:read", "event:write"},
			want:     false,
		},
		"no scopes required": {
			granted:  "org:read",
			required: nil,
			want:     true,
		},
		"granted is comma-separated (GitHub's format) and covers required": {
			granted:  "repo,security_events",
			required: []string{"repo", "security_events"},
			want:     true,
		},
		"granted is comma-separated and missing a required scope": {
			granted:  "repo",
			required: []string{"repo", "security_events"},
			want:     false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, oauthconn.HasScopes(tc.granted, tc.required))
		})
	}
}
