package auth_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gotrue "github.com/supabase-community/auth-go"
	"github.com/supabase-community/auth-go/types"

	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

// countingClient wraps a gotrue.Client and counts GetUser calls, with an
// artificial delay so concurrent callers actually overlap in tests.
type countingClient struct {
	gotrue.Client
	calls *atomic.Int32
	delay time.Duration
}

func (c countingClient) WithToken(token string) gotrue.Client {
	return countingClient{
		Client: c.Client.WithToken(token),
		calls:  c.calls,
		delay:  c.delay,
	}
}

func (c countingClient) GetUser() (*types.UserResponse, error) {
	c.calls.Add(1)
	time.Sleep(c.delay)
	return c.Client.GetUser()
}

// TestResolveUserCoalescesConcurrentCalls guards against the thundering herd
// behind issue #852: opening several tabs at once used to fire one GoTrue
// GetUser call per tab for the same (cache-miss) access token.
func TestResolveUserCoalescesConcurrentCalls(t *testing.T) {
	var calls atomic.Int32
	client := countingClient{
		Client: mocks.NewMockedGoTrueClient(),
		calls:  &calls,
		delay:  20 * time.Millisecond,
	}

	service := auth.NewService(testhelper.NewTestConfig(), client, nil)

	const tabs = 10
	var wg sync.WaitGroup
	wg.Add(tabs)
	for range tabs {
		go func() {
			defer wg.Done()
			_, err := service.ResolveToken(context.Background(), "access")
			assert.NoError(t, err)
		}()
	}
	wg.Wait()

	require.Equal(t, int32(1), calls.Load())
}
