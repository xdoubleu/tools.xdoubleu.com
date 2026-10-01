package main

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/internal/mcptools"
)

// addObsTool registers an observability tool: requireObservability, then the
// proto response marshalled to JSON text.
func addObsTool[In any](
	srv *mcp.Server,
	name, description string,
	produce func(context.Context, In) (proto.Message, error),
) {
	registerObsTool(srv, name, description, produce, mcptools.Result)
}

// addObsToolWithDefaults is addObsTool emitting zero-valued fields, so an
// empty list or zero count is explicit rather than absent.
func addObsToolWithDefaults[In any](
	srv *mcp.Server,
	name, description string,
	produce func(context.Context, In) (proto.Message, error),
) {
	registerObsTool(srv, name, description, produce, mcptools.ResultWithDefaults)
}

func registerObsTool[In any](
	srv *mcp.Server,
	name, description string,
	produce func(context.Context, In) (proto.Message, error),
	result func(proto.Message) (*mcp.CallToolResult, any, error),
) {
	//nolint:exhaustruct // name/description are the only fields tools need
	mcp.AddTool(srv, &mcp.Tool{Name: name, Description: description},
		func(
			ctx context.Context,
			_ *mcp.CallToolRequest,
			args In,
		) (*mcp.CallToolResult, any, error) {
			if err := requireObservability(ctx); err != nil {
				return nil, nil, err
			}
			msg, err := produce(ctx, args)
			if err != nil {
				return nil, nil, err
			}
			return result(msg)
		})
}
