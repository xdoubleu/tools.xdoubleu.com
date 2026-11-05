package runner_test

import (
	"context"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProxy is a ProxyListener that records the gateway and port it was given.
type fakeProxy struct {
	gateway *string
	port    *int
	err     error
	onStop  func()
}

func (f *fakeProxy) Listen(gateway string, port int) (func(), string, error) {
	if f.err != nil {
		return nil, "", f.err
	}
	*f.gateway = gateway
	if f.port != nil {
		*f.port = port
	}
	return f.onStop, net.JoinHostPort(gateway, strconv.Itoa(port)), nil
}

func TestTheVMLearnsWhereTheProxyListens(t *testing.T) {
	var envBody string
	exec := newExecScript(routesAnswer)
	exec.onExec = func(argv []string, _ string) {
		for i, a := range argv {
			if a == "--env-file" {
				body, err := os.ReadFile(argv[i+1])
				require.NoError(t, err)
				envBody = string(body)
			}
		}
	}
	var gateway string
	var port int
	box := sandboxWith(t, exec)
	box.ProxyPort = 8118
	box.Proxy = &fakeProxy{gateway: &gateway, err: nil, port: &port, onStop: func() {}}

	_, err := box.Run(context.Background(), spec(t, io.Discard))
	require.NoError(t, err)
	assert.Equal(t, 8118, port, "the sandbox picks the port it also tells the VM")
	assert.Contains(t, envBody, "FACTORY_PROXY=http://10.231.7.1:8118\n")

	envBody = ""
	box.Proxy = nil
	_, err = box.Run(context.Background(), spec(t, io.Discard))
	require.NoError(t, err)
	assert.False(t, strings.Contains(envBody, "FACTORY_PROXY"), envBody)
}
