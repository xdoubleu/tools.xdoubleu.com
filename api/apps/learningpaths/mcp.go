package learningpaths

import (
	"context"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
	"tools.xdoubleu.com/internal/mcptools"
)

const mcpAppName = "learningpaths"

// This app deliberately breaks the "MCP app tools are read-only" convention
// (see api/CLAUDE.md, adr-0023): agent-authored curricula is the feature, not
// an add-on, so learningpaths_create_path/update_path/record_progress below
// mutate. Every mutating tool still goes through RequireAppAccess and, like
// every read tool, only ever touches the calling user's own data — the
// connect handlers it wraps derive userID from context (getUser), never from
// a tool argument, so there is no way for a caller to address another user's
// path. See docs/adr-0023-learningpaths-mcp-write-tools.md for the full
// boundary and the explicit non-precedent statement.

type mcpListPathsArgs struct {
	Limit  int32 `json:"limit,omitempty"  jsonschema:"max paths to return"`
	Offset int32 `json:"offset,omitempty" jsonschema:"pagination offset"`
}

type mcpPathIDArgs struct {
	ID string `json:"id" jsonschema:"learning path id"`
}

type mcpItemArg struct {
	Type        string `json:"type,omitempty"      jsonschema:"item type, e.g. read/do"`
	Description string `json:"description"         jsonschema:"item description"`
	Completed   bool   `json:"completed,omitempty" jsonschema:"already completed"`
}

type mcpModuleArg struct {
	Title string       `json:"title"           jsonschema:"module title"`
	Items []mcpItemArg `json:"items,omitempty" jsonschema:"ordered items"`
}

type mcpResourceArg struct {
	Text string `json:"text" jsonschema:"freeform resource text, e.g. a URL"`
}

type mcpCreatePathArgs struct {
	Title     string           `json:"title"               jsonschema:"path title"`
	Goal      string           `json:"goal,omitempty"      jsonschema:"path goal"`
	Routine   string           `json:"routine,omitempty"   jsonschema:"recurring routine"`
	Modules   []mcpModuleArg   `json:"modules,omitempty"   jsonschema:"ordered modules"`
	Resources []mcpResourceArg `json:"resources,omitempty" jsonschema:"freeform resources"`
}

type mcpUpdatePathArgs struct {
	ID        string           `json:"id"                  jsonschema:"path id"`
	Title     string           `json:"title"               jsonschema:"path title"`
	Goal      string           `json:"goal,omitempty"      jsonschema:"path goal"`
	Routine   string           `json:"routine,omitempty"   jsonschema:"recurring routine"`
	Modules   []mcpModuleArg   `json:"modules,omitempty"   jsonschema:"replaces prior"`
	Resources []mcpResourceArg `json:"resources,omitempty" jsonschema:"replaces prior"`
}

type mcpRecordProgressArgs struct {
	ItemID    string `json:"item_id"   jsonschema:"item id, from learningpaths_get_path"`
	Completed bool   `json:"completed" jsonschema:"mark complete/incomplete"`
}

// RegisterMCPTools exposes the learningpaths app's RPCs on the combined apps
// MCP server: three read tools registered the same way every other app
// registers its read tools, plus three mutating tools that are this app's
// deliberate exception to the read-only convention (see the package-level
// comment above and adr-0023).
func (a *LearningPaths) RegisterMCPTools(srv *mcp.Server) {
	h := &learningPathsConnectHandler{app: a}

	mcptools.AddReadTool(srv, mcpAppName, "learningpaths_list_paths",
		"All learning paths owned by the calling user.", h.mcpListPaths)
	mcptools.AddReadTool(srv, mcpAppName, "learningpaths_get_path",
		"A single learning path with its modules, items, and resources.",
		h.mcpGetPath)
	mcptools.AddReadTool(srv, mcpAppName, "learningpaths_get_progress",
		"Completion counts for a learning path, overall and per module.",
		h.mcpGetProgress)

	addWriteTool(srv, "learningpaths_create_path",
		"Creates a new learning path with its modules, items, and resources. "+
			"Mutating — see this app's ADR for why.",
		h.mcpCreatePath)
	addWriteTool(srv, "learningpaths_update_path",
		"Wholesale-replaces a learning path's title/goal/routine/modules/"+
			"resources (the full tree, not a partial patch). Mutating — see "+
			"this app's ADR for why.",
		h.mcpUpdatePath)
	addWriteTool(srv, "learningpaths_record_progress",
		"Marks a single item complete or incomplete without resending the "+
			"whole tree. Mutating — see this app's ADR for why.",
		h.mcpRecordProgress)
}

// addWriteTool registers one mutating tool. It mirrors mcptools.AddReadTool
// exactly (same RequireAppAccess gate, same JSON result marshaling) but stays
// local to this file rather than becoming a second exported helper in
// internal/mcptools — this app is the only one with mutating MCP tools, and
// generalizing the helper before a second caller exists would be speculative
// (see this issue's "Not this").
func addWriteTool[In any](
	srv *mcp.Server,
	name, description string,
	produce func(context.Context, In) (proto.Message, error),
) {
	//nolint:exhaustruct // name/description are the only fields tools need
	mcp.AddTool(srv, &mcp.Tool{Name: name, Description: description},
		func(
			ctx context.Context,
			_ *mcp.CallToolRequest,
			args In,
		) (*mcp.CallToolResult, any, error) {
			if err := mcptools.RequireAppAccess(ctx, mcpAppName); err != nil {
				return nil, nil, err
			}
			msg, err := produce(ctx, args)
			if err != nil {
				return nil, nil, err
			}
			return mcptools.Result(msg)
		})
}

func toProtoModules(in []mcpModuleArg) []*learningpathsv1.Module {
	modules := make([]*learningpathsv1.Module, len(in))
	for i, m := range in {
		items := make([]*learningpathsv1.Item, len(m.Items))
		for j, it := range m.Items {
			items[j] = &learningpathsv1.Item{
				Type:        it.Type,
				Description: it.Description,
				Completed:   it.Completed,
			}
		}
		modules[i] = &learningpathsv1.Module{
			Title: m.Title,
			Items: items,
		}
	}
	return modules
}

func toProtoResources(in []mcpResourceArg) []*learningpathsv1.Resource {
	resources := make([]*learningpathsv1.Resource, len(in))
	for i, r := range in {
		resources[i] = &learningpathsv1.Resource{
			Text: r.Text,
		}
	}
	return resources
}

func (h *learningPathsConnectHandler) mcpListPaths(
	ctx context.Context, args mcpListPathsArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListLearningPaths(ctx, connect.NewRequest(
		&learningpathsv1.ListLearningPathsRequest{
			Limit:  args.Limit,
			Offset: args.Offset,
		},
	)))
}

func (h *learningPathsConnectHandler) mcpGetPath(
	ctx context.Context, args mcpPathIDArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetLearningPath(ctx, connect.NewRequest(
		&learningpathsv1.GetLearningPathRequest{Id: args.ID},
	)))
}

func (h *learningPathsConnectHandler) mcpGetProgress(
	ctx context.Context, args mcpPathIDArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetLearningPathProgress(ctx, connect.NewRequest(
		&learningpathsv1.GetLearningPathProgressRequest{Id: args.ID},
	)))
}

func (h *learningPathsConnectHandler) mcpCreatePath(
	ctx context.Context, args mcpCreatePathArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.CreateLearningPath(ctx, connect.NewRequest(
		&learningpathsv1.CreateLearningPathRequest{
			Title:     args.Title,
			Goal:      args.Goal,
			Routine:   args.Routine,
			Modules:   toProtoModules(args.Modules),
			Resources: toProtoResources(args.Resources),
		},
	)))
}

func (h *learningPathsConnectHandler) mcpUpdatePath(
	ctx context.Context, args mcpUpdatePathArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.UpdateLearningPath(ctx, connect.NewRequest(
		&learningpathsv1.UpdateLearningPathRequest{
			Id:        args.ID,
			Title:     args.Title,
			Goal:      args.Goal,
			Routine:   args.Routine,
			Modules:   toProtoModules(args.Modules),
			Resources: toProtoResources(args.Resources),
		},
	)))
}

func (h *learningPathsConnectHandler) mcpRecordProgress(
	ctx context.Context, args mcpRecordProgressArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.RecordItemProgress(ctx, connect.NewRequest(
		&learningpathsv1.RecordItemProgressRequest{
			ItemId:    args.ItemID,
			Completed: args.Completed,
		},
	)))
}
