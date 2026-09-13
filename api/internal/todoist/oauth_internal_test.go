package todoist

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOAuthConfig(t *testing.T) {
	conf := OAuthConfig("client-id", "client-secret", "https://api.example.com")

	assert.Equal(t, "client-id", conf.ClientID)
	assert.Equal(t, "client-secret", conf.ClientSecret)
	assert.Equal(t, "https://app.todoist.com/oauth/authorize", conf.Endpoint.AuthURL)
	assert.Equal(
		t,
		"https://api.todoist.com/oauth/access_token",
		conf.Endpoint.TokenURL,
	)
	assert.Equal(
		t, "https://api.example.com/learningpaths/oauth/todoist/callback",
		conf.RedirectURL,
	)
	assert.Equal(t, []string{"task:add"}, conf.Scopes)
}
