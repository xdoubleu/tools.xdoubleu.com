package connecttools

import (
	"context"
	"net/http"

	"connectrpc.com/connect/v2"
)

// RequestCookie returns the named cookie's value from the request a Connect
// handler is serving, and whether it was sent.
func RequestCookie(ctx context.Context, name string) (string, bool) {
	info, ok := connect.CallInfoForServerContext(ctx)
	if !ok {
		return "", false
	}
	cookie, err := (&http.Request{
		Header: http.Header{"Cookie": {info.RequestHeader().Get("Cookie")}},
	}).Cookie(name)
	if err != nil {
		return "", false
	}
	return cookie.Value, true
}

// RequestHeader returns a header of the request a Connect handler is serving.
func RequestHeader(ctx context.Context, name string) string {
	info, ok := connect.CallInfoForServerContext(ctx)
	if !ok {
		return ""
	}
	return info.RequestHeader().Get(name)
}
