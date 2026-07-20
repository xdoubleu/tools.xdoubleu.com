// Package mcptools holds the shared building blocks for the read-only MCP
// servers: the per-app access gate, a generic read-tool registrar, and the
// proto-to-JSON result marshaler. Both the observability MCP server and the
// per-app MCP tools use these so tool bodies stay a thin wrapper over the
// existing read RPCs.
package mcptools

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/models"
)

// NoArgs is the empty input for tools that take no parameters. It is a struct
// so its inferred JSON schema is an object, as the MCP spec requires.
type NoArgs struct{}

// RequireAppAccess gates a per-app read tool. It mirrors auth.AppAccess: the
// user resolved onto the request context must be an admin or have the app in
// their app-access list. Because the wrapped read handlers scope every query to
// that user's own ID, the tool grants exactly the access the caller already has
// over HTTP — no more.
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

// AddReadTool registers one read-only tool on srv. It applies the app-access
// gate uniformly and marshals the produced proto message to JSON text content,
// so each app's tool body is just the underlying read call.
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

// Unwrap adapts a Connect read handler call — `(*connect.Response[T], error)` —
// into the `(proto.Message, error)` a tool producer returns, so each per-app
// producer is a one-liner over the existing handler method. The response body of
// every Connect RPC is a proto message, so the assertion always holds.
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
