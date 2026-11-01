// Package mcptools holds shared MCP building blocks: the per-app access gate,
// read and write tool registrars, and the proto-to-JSON result marshaler.
package mcptools

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"connectrpc.com/connect/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	"tools.xdoubleu.com/internal/models"
)

// NoArgs is the empty tool input; a struct so its schema is an object, as MCP
// requires.
type NoArgs struct{}

// RequireAppAccess mirrors auth.AppAccess: admin, or the app in the user's
// app access. Read handlers scope queries to the user, so a tool grants
// exactly the caller's HTTP access.
func RequireAppAccess(ctx context.Context, appName string) error {
	user := contexttools.GetValue[models.User](ctx, constants.UserContextKey)
	if user != nil &&
		(user.Role == models.RoleAdmin || slices.Contains(user.AppAccess, appName)) {
		return nil
	}
	return connect.NewError(
		connect.CodePermissionDenied,
		"access to "+appName+" required",
	)
}

// RequireAgent admits only an agent account, not an admin or a user: agents
// never act as a person, so a person's session gets no agent-only tool.
func RequireAgent(ctx context.Context) error {
	user := contexttools.GetValue[models.User](ctx, constants.UserContextKey)
	if user != nil && user.Role == models.RoleAgent {
		return nil
	}
	return connect.NewError(
		connect.CodePermissionDenied,
		"this tool is for agent accounts only",
	)
}

// AddReadTool registers a read-only tool with the app-access gate and JSON
// result marshaling.
func AddReadTool[In any](
	srv *mcp.Server,
	appName, name, description string,
	produce func(context.Context, In) (proto.Message, error),
) {
	addTool(srv, name, description, produce, func(ctx context.Context) error {
		return RequireAppAccess(ctx, appName)
	})
}

// AddAgentReadTool registers a read-only tool for agent accounts only.
func AddAgentReadTool[In any](
	srv *mcp.Server,
	name, description string,
	produce func(context.Context, In) (proto.Message, error),
) {
	addTool(srv, name, description, produce, RequireAgent)
}

// AddAgentWriteTool registers a mutating tool for agent accounts only. Like
// AddWriteTool, each app's mutating tools need their own ADR.
func AddAgentWriteTool[In any](
	srv *mcp.Server,
	name, description string,
	produce func(context.Context, In) (proto.Message, error),
) {
	addTool(srv, name, description, produce, RequireAgent)
}

// AddWriteTool registers a mutating tool behind the same gate as AddReadTool.
// Each app's mutating tools need their own ADR (adr-0023, adr-0026).
func AddWriteTool[In any](
	srv *mcp.Server,
	appName, name, description string,
	produce func(context.Context, In) (proto.Message, error),
) {
	addTool(srv, name, description, produce, func(ctx context.Context) error {
		return RequireAppAccess(ctx, appName)
	})
}

func addTool[In any](
	srv *mcp.Server,
	name, description string,
	produce func(context.Context, In) (proto.Message, error),
	gate func(context.Context) error,
) {
	//nolint:exhaustruct // name/description are the only fields tools need
	mcp.AddTool(srv, &mcp.Tool{Name: name, Description: description},
		func(
			ctx context.Context,
			_ *mcp.CallToolRequest,
			args In,
		) (*mcp.CallToolResult, any, error) {
			if err := gate(ctx); err != nil {
				return nil, nil, err
			}
			msg, err := produce(ctx, args)
			if err != nil {
				return nil, nil, err
			}
			return Result(msg)
		})
}

// Unwrap adapts a Connect handler's (*connect.Response[T], error) into
// (proto.Message, error).
func Unwrap[T any](resp *T, err error) (proto.Message, error) {
	if err != nil {
		return nil, err
	}
	msg, ok := any(resp).(proto.Message)
	if !ok {
		return nil, fmt.Errorf("connect response %T is not a proto.Message", resp)
	}
	return msg, nil
}

// Result marshals a proto response to a tool result, as JSON text content and
// as structured output (raw protojson, embedded by the SDK).
func Result(msg proto.Message) (*mcp.CallToolResult, any, error) {
	return marshalResult(protojson.MarshalOptions{}, msg)
}

// ResultWithDefaults is Result with zero-valued fields emitted.
func ResultWithDefaults(msg proto.Message) (*mcp.CallToolResult, any, error) {
	return marshalResult(protojson.MarshalOptions{EmitUnpopulated: true}, msg)
}

func marshalResult(
	opts protojson.MarshalOptions, msg proto.Message,
) (*mcp.CallToolResult, any, error) {
	data, err := opts.Marshal(msg)
	if err != nil {
		return nil, nil, err
	}
	//nolint:exhaustruct // only Content carries the tool output
	return &mcp.CallToolResult{
		//nolint:exhaustruct // TextContent needs only Text
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, json.RawMessage(data), nil
}
