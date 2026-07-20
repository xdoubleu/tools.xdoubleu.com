package icsproxy

import (
	"context"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	icsproxyv1 "tools.xdoubleu.com/gen/icsproxy/v1"
	"tools.xdoubleu.com/internal/mcptools"
)

const mcpAppName = "icsproxy"

type mcpTokenArgs struct {
	Token string `json:"token" jsonschema:"filter-config token"`
}

type mcpSourceURLArgs struct {
	SourceURL string `json:"source_url" jsonschema:"upstream ICS URL to preview"`
}

// RegisterMCPTools exposes the icsproxy app's read-only RPCs on the combined
// apps MCP server. Every tool returns the calling user's own filter configs.
func (a *ICSProxy) RegisterMCPTools(srv *mcp.Server) {
	h := &icsProxyConnectHandler{app: a}

	mcptools.AddReadTool(srv, mcpAppName, "icsproxy_list_configs",
		"The user's ICS filter configs.", h.mcpListConfigs)
	mcptools.AddReadTool(srv, mcpAppName, "icsproxy_get_config",
		"A single filter config plus its currently-resolved events.",
		h.mcpGetConfig)
	mcptools.AddReadTool(srv, mcpAppName, "icsproxy_preview_events",
		"Events parsed from an upstream ICS URL (no config saved).",
		h.mcpPreviewEvents)
}

func (h *icsProxyConnectHandler) mcpListConfigs(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListConfigs(ctx, connect.NewRequest(
		&icsproxyv1.ListConfigsRequest{},
	)))
}

func (h *icsProxyConnectHandler) mcpGetConfig(
	ctx context.Context, args mcpTokenArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetConfig(ctx, connect.NewRequest(
		&icsproxyv1.GetConfigRequest{Token: args.Token},
	)))
}

func (h *icsProxyConnectHandler) mcpPreviewEvents(
	ctx context.Context, args mcpSourceURLArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.PreviewEvents(ctx, connect.NewRequest(
		&icsproxyv1.PreviewEventsRequest{SourceUrl: args.SourceURL},
	)))
}
