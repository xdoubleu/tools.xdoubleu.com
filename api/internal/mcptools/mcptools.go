// Package mcptools holds shared MCP building blocks: the per-app access gate,
// a read-tool registrar, and the proto-to-JSON result marshaler.
package mcptools

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"connectrpc.com/connect"
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
		errors.New("access to "+appName+" required"),
	)
}

// AddReadTool registers a read-only tool with the app-access gate and JSON
// result marshaling.
func AddReadTool[In any](
	srv *mcp.Server,
	appName, name, description string,
	produce func(context.Context, In) (proto.Message, error),
) {
	//nolint:exhaustruct // name/description are the only fields tools need
	mcp.AddTool(srv, &mcp.Tool{Name: name, Description: description},
		func(
			ctx context.Context,
			_ *mcp.CallToolRequest,
			args In,
		) (*mcp.CallToolResult, any, error) {
			if err := RequireAppAccess(ctx, appName); err != nil {
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
func Unwrap[T any](resp *connect.Response[T], err error) (proto.Message, error) {
	if err != nil {
		return nil, err
	}
	msg, ok := any(resp.Msg).(proto.Message)
	if !ok {
		return nil, fmt.Errorf("connect response %T is not a proto.Message", resp.Msg)
	}
	return msg, nil
}

// Result marshals a proto response to JSON text content for a tool result.
func Result(msg proto.Message) (*mcp.CallToolResult, any, error) {
	data, err := protojson.Marshal(msg)
	if err != nil {
		return nil, nil, err
	}
	//nolint:exhaustruct // only Content carries the tool output
	return &mcp.CallToolResult{
		//nolint:exhaustruct // TextContent needs only Text
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil, nil
}
