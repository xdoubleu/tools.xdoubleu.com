package mcptools

import (
	"context"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	"tools.xdoubleu.com/internal/models"
)

// AgentToolPrefix starts the name of every tool an agent account may use.
const AgentToolPrefix = "factory_"

var errUnknownTool = errors.New("unknown tool")

// AgentToolsOnly limits an agent account to the factory_ tools, whatever the
// per-app gates say: other tools are left out of its tool list and refused when
// called, as if they didn't exist, and resources and prompts are refused. Every
// other caller's tool list leaves the factory_ tools out, since they are for
// agents alone (RequireAgent refuses their calls).
func AgentToolsOnly() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(
			ctx context.Context, method string, req mcp.Request,
		) (mcp.Result, error) {
			user := contexttools.GetValue[models.User](ctx, constants.UserContextKey)
			if user == nil || user.Role != models.RoleAgent {
				return hideAgentTools(ctx, method, req, next)
			}

			switch {
			case strings.HasPrefix(method, "resources/"),
				strings.HasPrefix(method, "prompts/"):
				return nil, errUnknownTool
			case method == "tools/call":
				call, ok := req.(*mcp.CallToolRequest)
				if !ok || call.Params == nil ||
					!strings.HasPrefix(call.Params.Name, AgentToolPrefix) {
					return nil, errUnknownTool
				}
			case method == "tools/list":
				res, err := next(ctx, method, req)
				if list, ok := res.(*mcp.ListToolsResult); ok && err == nil {
					list.Tools = factoryTools(list.Tools)
				}
				return res, err
			}
			return next(ctx, method, req)
		}
	}
}

// hideAgentTools leaves the factory_ tools out of a non-agent's tool list.
func hideAgentTools(
	ctx context.Context, method string, req mcp.Request, next mcp.MethodHandler,
) (mcp.Result, error) {
	res, err := next(ctx, method, req)
	if list, ok := res.(*mcp.ListToolsResult); ok && err == nil && method == "tools/list" {
		kept := make([]*mcp.Tool, 0, len(list.Tools))
		for _, t := range list.Tools {
			if !strings.HasPrefix(t.Name, AgentToolPrefix) {
				kept = append(kept, t)
			}
		}
		list.Tools = kept
	}
	return res, err
}

func factoryTools(tools []*mcp.Tool) []*mcp.Tool {
	kept := make([]*mcp.Tool, 0, len(tools))
	for _, t := range tools {
		if strings.HasPrefix(t.Name, AgentToolPrefix) {
			kept = append(kept, t)
		}
	}
	return kept
}
