package digitalocean

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// DigitalOcean hands live_url back with an http(s) scheme even though it is a
// websocket endpoint, so it has to be rewritten before dialling — production
// URLs are https, which must become wss.
func TestLiveLogURL(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		expected string
	}{
		{
			name:     "https becomes wss",
			raw:      "https://api.digitalocean.com/logs?token=abc",
			expected: "wss://api.digitalocean.com/logs?token=abc",
		},
		{
			name:     "http becomes ws",
			raw:      "http://127.0.0.1:8080/live",
			expected: "ws://127.0.0.1:8080/live",
		},
		{
			name:     "wss is left alone",
			raw:      "wss://api.digitalocean.com/logs",
			expected: "wss://api.digitalocean.com/logs",
		},
		{
			name:     "unparseable url is passed through",
			raw:      "://nonsense",
			expected: "://nonsense",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, liveLogURL(tc.raw))
		})
	}
}
