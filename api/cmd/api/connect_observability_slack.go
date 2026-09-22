package main

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"tools.xdoubleu.com/internal/slackwebhook"
)

// notifySlack backs the notify_slack MCP tool — the third deliberate
// mutation in this otherwise read-only observability surface, alongside
// resolveSentryIssue and dismissSecurityAlert (api/AGENTS.md's "Apps MCP
// Server" section). Unlike those two, it has no Connect RPC/proto message
// of its own (issue #1628's scope is MCP-tool-only), so it returns
// *emptypb.Empty rather than a generated observabilityv1 response type.
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
