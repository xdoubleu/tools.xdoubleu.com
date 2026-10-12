package testhelper

import (
	"context"

	"connectrpc.com/connect/v2"
)

// WithCookie returns ctx carrying cookie as the request Cookie header of the
// next ConnectRPC client call.
func WithCookie(ctx context.Context, cookie string) context.Context {
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", cookie)
	return ctx
}
