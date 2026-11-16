package mcptools

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	"tools.xdoubleu.com/internal/models"
)

// factoryApp owns the factory_ tools, which only agents may use.
const factoryApp = "factory"

var errUnknownTool = errors.New("unknown tool")

// AgentGrants limits an agent account to the apps it holds a grant on (adr-0047).
// A tool belongs to an app by its "<app>_" prefix, a resource by its "<app>://"
// scheme; anything else (admin tools, prompts, an app without a grant) is left
// out of the lists and refused as if it didn't exist. Other callers' tool lists
// leave out the factory_ tools, which are for agents alone.
func AgentGrants() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(
			ctx context.Context, method string, req mcp.Request,
		) (mcp.Result, error) {
			user := contexttools.GetValue[models.User](ctx, constants.UserContextKey)
			if user == nil || user.Role != models.RoleAgent {
				return hideAgentTools(ctx, method, req, next)
			}
			return agentCall(ctx, user.AppAccess, method, req, next)
		}
	}
}

func agentCall(
	ctx context.Context, apps []string, method string, req mcp.Request,
	next mcp.MethodHandler,
) (mcp.Result, error) {
	switch method {
	case "tools/call":
		call, ok := req.(*mcp.CallToolRequest)
		if !ok || call.Params == nil || !toolHeld(apps, call.Params.Name) {
			return nil, errUnknownTool
		}
	case "resources/read":
		read, ok := req.(*mcp.ReadResourceRequest)
		if !ok || read.Params == nil || !held(apps, read.Params.URI, "://") {
			return nil, errUnknownTool
		}
	case "tools/list":
		res, err := next(ctx, method, req)
		if list, ok := res.(*mcp.ListToolsResult); ok && err == nil {
			list.Tools = slices.DeleteFunc(slices.Clone(list.Tools), func(t *mcp.Tool) bool {
				return !toolHeld(apps, t.Name)
			})
		}
		return res, err
	case "resources/list":
		res, err := next(ctx, method, req)
		if list, ok := res.(*mcp.ListResourcesResult); ok && err == nil {
			list.Resources = slices.DeleteFunc(
				slices.Clone(list.Resources), func(r *mcp.Resource) bool {
					return !held(apps, r.URI, "://")
				})
		}
		return res, err
	default:
		if strings.HasPrefix(method, "resources/") ||
			strings.HasPrefix(method, "prompts/") ||
			strings.HasPrefix(method, "completion/") {
			return nil, errUnknownTool
		}
	}
	return next(ctx, method, req)
}

// toolHeld reports whether an agent holding apps may use the tool.
func toolHeld(apps []string, name string) bool {
	return held(apps, name, "_") || agentMayObserve(apps, name)
}

// held reports whether name starts with "<app><sep>" for an app in apps.
func held(apps []string, name, sep string) bool {
	return slices.ContainsFunc(apps, func(app string) bool {
		return strings.HasPrefix(name, app+sep)
	})
}

// hideAgentTools leaves the factory_ tools out of a non-agent's tool list.
func hideAgentTools(
	ctx context.Context, method string, req mcp.Request, next mcp.MethodHandler,
) (mcp.Result, error) {
	res, err := next(ctx, method, req)
	if list, ok := res.(*mcp.ListToolsResult); ok && err == nil && method == "tools/list" {
		list.Tools = slices.DeleteFunc(slices.Clone(list.Tools), func(t *mcp.Tool) bool {
			return strings.HasPrefix(t.Name, factoryApp+"_")
		})
	}
	return res, err
}
