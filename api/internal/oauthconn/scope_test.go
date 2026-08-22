package oauthconn_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/oauthconn"
)

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
