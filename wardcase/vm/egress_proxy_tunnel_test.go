package vm_test

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/vm"
)

// connect asks the proxy for a tunnel to host and returns its status line and,
// when it opened, what the far end echoes.
func connect(t *testing.T, p *vm.EgressProxy, host string) (string, string) {
	t.Helper()
	srv := httptest.NewServer(p)
	t.Cleanup(srv.Close)
	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	require.NoError(t, err)
	defer conn.Close()
	_, err = io.WriteString(conn, "CONNECT "+host+" HTTP/1.1\r\nHost: "+host+"\r\n\r\n")
	require.NoError(t, err)
	rd := bufio.NewReader(conn)
	status, err := rd.ReadString('\n')
	require.NoError(t, err)
	if status != "HTTP/1.1 200 Connection Established\r\n" {
		return status, ""
	}
	_, err = rd.ReadString('\n')
	require.NoError(t, err)
	_, err = io.WriteString(conn, "ping\n")
	require.NoError(t, err)
	echo, err := rd.ReadString('\n')
	require.NoError(t, err)
	return status, echo
}

func tunnelProxy(t *testing.T, hosts []string) (*vm.EgressProxy, *[]string) {
	t.Helper()
	dialed := &[]string{}
	p, _ := proxyFor(t, func(c *vm.EgressConfig) {
		c.FetchHosts = hosts
		c.Dial = func(_ context.Context, _, addr string) (net.Conn, error) {
			*dialed = append(*dialed, addr)
			a, b := net.Pipe()
			go func() {
				defer b.Close()
				line, _ := bufio.NewReader(b).ReadString('\n')
				_, _ = io.WriteString(b, line)
			}()
			return a, nil
		}
	})
	return p, dialed
}

func TestAConnectToAFetchHostIsTunnelled(t *testing.T) {
	p, dialed := tunnelProxy(t, []string{"github.com:443"})

	status, echo := connect(t, p, "github.com:443")

	assert.Contains(t, status, "200")
	assert.Equal(t, "ping\n", echo)
	assert.Equal(t, []string{"github.com:443"}, *dialed)
}

func TestAConnectToAnyOtherHostIsRefused(t *testing.T) {
	p, dialed := tunnelProxy(t, []string{"github.com:443"})

	status, _ := connect(t, p, "evil.example:443")

	assert.Contains(t, status, "403")
	assert.Empty(t, *dialed)
}

func TestTheDefaultFetchHostsCoverGitHubAndNixosTarballs(t *testing.T) {
	p, _ := tunnelProxy(t, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodConnect, "http://x", nil)
	req.Host = "tarballs.nixos.org:443"

	p.ServeHTTP(rec, req)

	assert.NotEqual(t, http.StatusForbidden, rec.Code)
}
