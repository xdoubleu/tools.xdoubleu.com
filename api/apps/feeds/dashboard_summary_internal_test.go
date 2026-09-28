package feeds

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFeedOrigin(t *testing.T) {
	cases := map[string]string{
		"https://u:p@example.com:8443/feed.xml?token=x": "https://example.com:8443",
		"http://example.com/private/abc123/rss":         "http://example.com",
		"mailto:someone@example.com":                    "",
		"":                                              "",
		"://bad":                                        "",
	}
	for in, want := range cases {
		assert.Equal(t, want, feedOrigin(in), in)
	}
}
