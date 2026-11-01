package connecttools_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/connecttools"
)

// probe is a RunnerService that records what the helpers read from the request
// it serves: the only way to get a Connect server context.
type probe struct {
	factoryv1connect.UnimplementedRunnerServiceHandler

	cookie    string
	hasCookie bool
	header    string
}

func (p *probe) Heartbeat(
	ctx context.Context, _ *factoryv1.HeartbeatRequest,
) (*factoryv1.HeartbeatResponse, error) {
	p.cookie, p.hasCookie = connecttools.RequestCookie(ctx, "accessToken")
	p.header = connecttools.RequestHeader(ctx, "Authorization")
	return &factoryv1.HeartbeatResponse{}, nil
}

// call sends one Heartbeat with the given Cookie and Authorization headers.
func call(t *testing.T, cookie, authorization string) *probe {
	t.Helper()
	stub := factoryv1connect.UnimplementedRunnerServiceHandler{}
	p := &probe{
		UnimplementedRunnerServiceHandler: stub,
		cookie:                            "",
		hasCookie:                         false,
		header:                            "",
	}
	server := connect.NewServer()
	factoryv1connect.RegisterRunnerServiceHandler(server, p)
	mux := http.NewServeMux()
	connecthttp.Mount(postMux{mux}, server)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client := factoryv1connect.NewRunnerServiceClient(connect.NewClient(
		connecthttp.NewTransport(ts.Client(), ts.URL, connecthttp.WithReadMaxBytes(0)),
	))
	ctx, info := connect.NewClientContext(context.Background())
	if cookie != "" {
		info.RequestHeader().Set("Cookie", cookie)
	}
	if authorization != "" {
		info.RequestHeader().Set("Authorization", authorization)
	}
	_, err := client.Heartbeat(ctx, &factoryv1.HeartbeatRequest{})
	require.NoError(t, err)
	return p
}

// postMux adapts a ServeMux to connecthttp.Mount's Handle method.
type postMux struct{ mux *http.ServeMux }

func (m postMux) Handle(pattern string, h http.Handler) {
	m.mux.Handle("POST "+pattern, h)
}

func TestRequestCookieAndHeaderReadTheServedRequest(t *testing.T) {
	p := call(t, "theme=dark; accessToken=abc123", "Bearer tok")

	assert.True(t, p.hasCookie)
	assert.Equal(t, "abc123", p.cookie)
	assert.Equal(t, "Bearer tok", p.header)
}

func TestRequestCookieIsAbsentWhenNotSent(t *testing.T) {
	p := call(t, "theme=dark", "")

	assert.False(t, p.hasCookie)
	assert.Empty(t, p.cookie)
	assert.Empty(t, p.header)
}

func TestRequestCookieAndHeaderOutsideAConnectCall(t *testing.T) {
	value, ok := connecttools.RequestCookie(context.Background(), "accessToken")
	assert.False(t, ok)
	assert.Empty(t, value)
	assert.Empty(t, connecttools.RequestHeader(context.Background(), "Authorization"))
}
