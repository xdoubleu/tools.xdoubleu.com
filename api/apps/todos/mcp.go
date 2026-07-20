package todos

import (
	"context"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	todosv1 "tools.xdoubleu.com/gen/todos/v1"
	"tools.xdoubleu.com/internal/mcptools"
)

const mcpAppName = "todos"

type mcpListTasksArgs struct {
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"workspace id"`
	SectionID   string `json:"section_id,omitempty"   jsonschema:"section filter"`
	Status      string `json:"status,omitempty"       jsonschema:"open|done|archived"`
}

type mcpTaskIDArgs struct {
	ID string `json:"id" jsonschema:"task id"`
}

type mcpSearchTasksArgs struct {
	Query       string `json:"query"                  jsonschema:"search query"`
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"workspace id"`
}

// RegisterMCPTools exposes the todos app's read-only RPCs on the combined apps
// MCP server. Every tool returns the calling user's own tasks and settings.
func (a *Todos) RegisterMCPTools(srv *mcp.Server) {
	taskH := &taskConnectHandler{app: a}
	settingsH := &settingsConnectHandler{app: a}

	mcptools.AddReadTool(srv, mcpAppName, "todos_list_tasks",
		"Tasks in a workspace/section, filtered by status.", taskH.mcpListTasks)
	mcptools.AddReadTool(srv, mcpAppName, "todos_get_task",
		"A single task with its subtasks and links.", taskH.mcpGetTask)
	mcptools.AddReadTool(srv, mcpAppName, "todos_search_tasks",
		"Search tasks by text, grouped into open/done/archived.",
		taskH.mcpSearchTasks)
	mcptools.AddReadTool(srv, mcpAppName, "todos_get_settings",
		"The user's todos settings: label presets, URL patterns, archive "+
			"settings, sections, policies.", settingsH.mcpGetSettings)
}

func (h *taskConnectHandler) mcpListTasks(
	ctx context.Context, args mcpListTasksArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListTasks(ctx, connect.NewRequest(
		&todosv1.ListTasksRequest{
			WorkspaceId: args.WorkspaceID,
			SectionId:   args.SectionID,
			Status:      args.Status,
		},
	)))
}

func (h *taskConnectHandler) mcpGetTask(
	ctx context.Context, args mcpTaskIDArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetTask(ctx, connect.NewRequest(
		&todosv1.GetTaskRequest{Id: args.ID},
	)))
}

func (h *taskConnectHandler) mcpSearchTasks(
	ctx context.Context, args mcpSearchTasksArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.SearchTasks(ctx, connect.NewRequest(
		&todosv1.SearchTasksRequest{
			Query:       args.Query,
			WorkspaceId: args.WorkspaceID,
		},
	)))
}

func (h *settingsConnectHandler) mcpGetSettings(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetSettings(ctx, connect.NewRequest(
		&todosv1.GetSettingsRequest{},
	)))
}
