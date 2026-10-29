package obs

import (
	"context"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	"tools.xdoubleu.com/internal/models"
)

// requireAdmin gates admin-only Connect RPCs.
func requireAdmin(ctx context.Context) error {
	user := contexttools.GetValue[models.User](ctx, constants.UserContextKey)
	if user.Role != models.RoleAdmin {
		return connect.NewError(
			connect.CodePermissionDenied,
			"admin access required",
		)
	}
	return nil
}

// requireObservability gates the observability MCP tools: admins and the
// service role, which has no access to user data.
func requireObservability(ctx context.Context) error {
	user := contexttools.GetValue[models.User](ctx, constants.UserContextKey)
	if user.Role != models.RoleAdmin && user.Role != models.RoleService {
		return connect.NewError(
			connect.CodePermissionDenied,
			"admin access required",
		)
	}
	return nil
}