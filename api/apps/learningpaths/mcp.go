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

// Unlike other apps, learningpaths exposes mutating MCP tools. They still go
// through RequireAppAccess and derive userID from context, never from a tool
// argument (docs/adr-0023-learningpaths-mcp-write-tools.md).

type mcpListPathsArgs struct {
	Limit  int32 `json:"limit,omitempty"  jsonschema:"max paths to return"`
	Offset int32 `json:"offset,omitempty" jsonschema:"pagination offset"`
}

type mcpPathIDArgs struct {
	ID string `json:"id" jsonschema:"learning path id"`
}

type mcpItemArg struct {
	Type        string `json:"type,omitempty"      jsonschema:"read/do/checkpoint..."`
	Description string `json:"description"         jsonschema:"what to do"`
	Completed   bool   `json:"completed,omitempty" jsonschema:"already completed"`
}

type mcpModuleArg struct {
	Title string       `json:"title"           jsonschema:"module title"`
	Items []mcpItemArg `json:"items,omitempty" jsonschema:"ordered items"`
}

type mcpResourceArg struct {
	Text string `json:"text" jsonschema:"freeform resource, e.g. URL"`
}

type mcpCreatePathArgs struct {
	Title     string           `json:"title"               jsonschema:"path title"`
	Goal      string           `json:"goal,omitempty"      jsonschema:"outcome sought"`
	Routine   string           `json:"routine,omitempty"   jsonschema:"recurring cadence"`
	Modules   []mcpModuleArg   `json:"modules,omitempty"   jsonschema:"ordered modules"`
	Resources []mcpResourceArg `json:"resources,omitempty" jsonschema:"freeform resources"`
}

type mcpUpdatePathArgs struct {
	ID        string           `json:"id"                  jsonschema:"path id"`
	Title     string           `json:"title"               jsonschema:"path title"`
	Goal      string           `json:"goal,omitempty"      jsonschema:"outcome sought"`
	Routine   string           `json:"routine,omitempty"   jsonschema:"recurring cadence"`
	Modules   []mcpModuleArg   `json:"modules,omitempty"   jsonschema:"replaces prior"`
	Resources []mcpResourceArg `json:"resources,omitempty" jsonschema:"replaces prior"`
}

type mcpRecordProgressArgs struct {
	ItemID    string `json:"item_id"   jsonschema:"item id, from learningpaths_get_path"`
	Completed bool   `json:"completed" jsonschema:"mark complete/incomplete"`
}

// RegisterMCPTools exposes three read and three write tools on the combined
// apps MCP server.
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
		mcpCreatePathDescription, h.mcpCreatePath)
	addWriteTool(srv, "learningpaths_update_path",
		mcpUpdatePathDescription, h.mcpUpdatePath)
	addWriteTool(srv, "learningpaths_record_progress",
		mcpRecordProgressDescription, h.mcpRecordProgress)

	registerAuthoringGuideResource(srv)
}

// registerAuthoringGuideResource serves the authoring guidebook as the
// learningpaths://authoring-guide resource. It holds no user data, so no
// app-access gate applies.
func registerAuthoringGuideResource(srv *mcp.Server) {
	//nolint:exhaustruct // name/URI/MIMEType/i18n are the fields this resource needs
	srv.AddResource(&mcp.Resource{
		Name:     "Learning paths authoring guide",
		URI:      authoringGuideURI,
		MIMEType: authoringGuideMIME,
		Description: "How to author, refine, and track a learning path on " +
			"tools.xdoubleu.com: the data model, the learningpaths_* MCP tools, " +
			"the create workflow, and the curation rubric.",
	}, authoringGuideHandler)
}

// authoringGuideHandler serves the authoring-guide resource; a named function
// so tests can call it directly.
func authoringGuideHandler(_ context.Context, _ *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	//nolint:exhaustruct // a read of static text sets only URI/MIMEType/Text
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{
		{
			URI:      authoringGuideURI,
			MIMEType: authoringGuideMIME,
			Text:     learningPathsAuthoringGuide,
		},
	}}, nil
}

// addWriteTool registers one mutating tool, mirroring mcptools.AddReadTool.
// Kept local: this is the only app with mutating tools.
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
