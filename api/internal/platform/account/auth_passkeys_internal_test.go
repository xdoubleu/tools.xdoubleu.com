package account

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientIP(t *testing.T) {
	// A public peer is its own address; its forwarding header is ignored.
	assert.Equal(t, "8.8.8.8", clientIP("8.8.8.8:1234", "1.2.3.4"))
	// Behind the proxy the last hop is the client.
	assert.Equal(t, "9.9.9.9", clientIP("127.0.0.1:1234", "1.2.3.4, 9.9.9.9"))
	// Without a usable peer address the raw value keys the lockout.
	assert.Equal(t, "garbage", clientIP("garbage", ""))
}
