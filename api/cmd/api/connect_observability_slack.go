package main

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"tools.xdoubleu.com/internal/slackwebhook"
)

// notifySlack backs the notify_slack MCP tool, a deliberate mutation. It has
// no Connect RPC, so it returns *emptypb.Empty.
func (h *obsConnectHandler) notifySlack(
	ctx context.Context,
	title, message string,
) (*emptypb.Empty, error) {
	if err := h.app.slackClient.Send(ctx, title, message); err != nil {
		if errors.Is(err, slackwebhook.ErrNotConfigured) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return &emptypb.Empty{}, nil
}
