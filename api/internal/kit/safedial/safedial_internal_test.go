package safedial

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDialer(t *testing.T) {
	dialer := newDialer(5 * time.Second)

	assert.Equal(t, 5*time.Second, dialer.Timeout)
	assert.Equal(t, 30*time.Second, dialer.KeepAlive)
}

func TestControl(t *testing.T) {
	tests := map[string]struct {
		address string
		wantErr bool
	}{
		"unparseable address, SplitHostPort fails": {
			address: "not-a-host-port",
			wantErr: true,
		},
		"private host, valid port": {
			address: "127.0.0.1:80",
			wantErr: true,
		},
		"public host, valid port": {
			address: "1.1.1.1:80",
			wantErr: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := control("", tt.address, nil)

			if !tt.wantErr {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.ErrorIs(t, err, ErrBlockedAddress)
		})
	}
}
