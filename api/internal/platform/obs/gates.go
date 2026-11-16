package obs

import (
	"context"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/mcptools"
)

// RequireAdmin gates admin-only Connect RPCs.
func RequireAdmin(ctx context.Context) error {
	user := contexttools.GetValue[models.User](ctx, constants.UserContextKey)
	if user.Role != models.RoleAdmin {
		return connect.NewError(
			connect.CodePermissionDenied,
			"admin access required",
		)
	}
	return nil
}

// RequireObservability gates the observability MCP tools: admins, the service
// role (no access to user data) and agents granted the tool (adr-0052).
func RequireObservability(ctx context.Context, tool string) error {
	if !mcptools.CanObserve(ctx, tool) {
		return connect.NewError(
			connect.CodePermissionDenied,
			"admin access required",
		)
	}
	return nil
}
