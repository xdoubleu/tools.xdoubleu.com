// Package mcp exposes the learningpaths tools on the apps MCP server.
package mcp

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	"tools.xdoubleu.com/apps/learningpaths/internal/protomap"
	"tools.xdoubleu.com/apps/learningpaths/internal/services"
	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/mcptools"
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
	Due         string `json:"due,omitempty"       jsonschema:"Todoist due string"`

	//nolint:gostruct //LinkedBook is read-only, never authored
	LinkedBookID *string `json:"linked_book_id,omitempty" jsonschema:"auto-completes item"`

	//nolint:lll //jsonschema description
	ExternalBook *mcpExternalBookArg `json:"external_book,omitempty" jsonschema:"book not in library, from books_search_external; added to To Learn and linked"`
}

type mcpExternalBookArg struct {
	Provider   string `json:"provider"    jsonschema:"search result provider"`
	ProviderID string `json:"provider_id" jsonschema:"search result provider_id"`
}

type mcpQuizQuestionArg struct {
	Prompt             string   `json:"prompt"               jsonschema:"question"`
	Options            []string `json:"options"              jsonschema:"choices"`
	CorrectAnswerIndex int32    `json:"correct_answer_index" jsonschema:"correct index"`
}

type mcpModuleArg struct {
	Title string               `json:"title,omitempty" jsonschema:"module title"`
	Items []mcpItemArg         `json:"items,omitempty" jsonschema:"ordered items"`
	Quiz  []mcpQuizQuestionArg `json:"quiz,omitempty"  jsonschema:"MCQ on checkpoint item"`
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

	//nolint:lll // struct tags
	ReminderSchedules map[string]string `json:"reminder_schedules,omitempty" jsonschema:"item type -> Todoist due string"`
}

type mcpUpdatePathArgs struct {
	ID        string           `json:"id"                  jsonschema:"path id"`
	Title     string           `json:"title"               jsonschema:"path title"`
	Goal      string           `json:"goal,omitempty"      jsonschema:"outcome sought"`
	Routine   string           `json:"routine,omitempty"   jsonschema:"recurring cadence"`
	Modules   []mcpModuleArg   `json:"modules,omitempty"   jsonschema:"replaces prior"`
	Resources []mcpResourceArg `json:"resources,omitempty" jsonschema:"replaces prior"`

	//nolint:lll // struct tags
	ReminderSchedules map[string]string `json:"reminder_schedules,omitempty" jsonschema:"replaces prior"`
}

type mcpRecordProgressArgs struct {
	ItemID    string `json:"item_id"   jsonschema:"item id, from learningpaths_get_path"`
	Completed bool   `json:"completed" jsonschema:"mark complete/incomplete"`
}

type tools struct {
	paths   *services.LearningPathService
	todoist *services.TodoistService
}

// Register exposes four read and three write tools on the combined apps MCP
// server. The write tools call the same service methods as their RPCs.
func Register(
	srv *mcp.Server,
	paths *services.LearningPathService,
	todoist *services.TodoistService,
) {
	h := &tools{paths: paths, todoist: todoist}

	mcptools.AddReadTool(srv, mcpAppName, "learningpaths_get_todoist_status",
		"A path's Todoist reminder state as stored (no Todoist call): connection, "+
			"requested scope and whether it needs a reconnect, the user's shared "+
			"project id, and the active module's items with their task ids.",
		h.mcpGetTodoistStatus)
	mcptools.AddReadTool(srv, mcpAppName, "learningpaths_list_paths",
		"All learning paths owned by the calling user.", h.mcpListPaths)
	mcptools.AddReadTool(srv, mcpAppName, "learningpaths_get_path",
		"A single learning path with its modules, items, and resources.",
		h.mcpGetPath)
	mcptools.AddReadTool(srv, mcpAppName, "learningpaths_get_progress",
		"Completion counts for a learning path, overall and per module.",
		h.mcpGetProgress)

	mcptools.AddWriteTool(srv, mcpAppName, "learningpaths_create_path",
		mcpCreatePathDescription, h.mcpCreatePath)
	mcptools.AddWriteTool(srv, mcpAppName, "learningpaths_update_path",
		mcpUpdatePathDescription, h.mcpUpdatePath)
	mcptools.AddWriteTool(srv, mcpAppName, "learningpaths_record_progress",
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

func userID(ctx context.Context) (string, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return "", connect.Errorf(
			connect.CodeUnauthenticated, "user not authenticated",
		)
	}
	return user.ID, nil
}

func parseID(raw, what string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, connect.Errorf(
			connect.CodeInvalidArgument, "invalid %s", what,
		)
	}
	return id, nil
}

func toModules(in []mcpModuleArg) []models.Module {
	modules := make([]models.Module, len(in))
	for i, m := range in {
		items := make([]models.Item, len(m.Items))
		for j, it := range m.Items {
			//nolint:exhaustruct //ID/ModuleID assigned by the repository
			items[j] = models.Item{
				Type:         it.Type,
				Description:  it.Description,
				SortOrder:    j,
				Completed:    it.Completed,
				Due:          it.Due,
				LinkedBookID: parseOptionalUUID(it.LinkedBookID),
				ExternalBook: toExternalBook(it.ExternalBook),
			}
		}
		//nolint:exhaustruct //ID/LearningPathID assigned by the repository
		modules[i] = models.Module{
			Title:     m.Title,
			SortOrder: i,
			Items:     items,
			Quiz:      toQuiz(m.Quiz),
		}
	}
	return modules
}

func toExternalBook(in *mcpExternalBookArg) *models.ExternalBookRef {
	if in == nil || in.Provider == "" || in.ProviderID == "" {
		return nil
	}
	return &models.ExternalBookRef{Provider: in.Provider, ProviderID: in.ProviderID}
}

func toQuiz(in []mcpQuizQuestionArg) []models.QuizQuestion {
	quiz := make([]models.QuizQuestion, len(in))
	for i, q := range in {
		quiz[i] = models.QuizQuestion{
			Prompt:             q.Prompt,
			Options:            q.Options,
			CorrectAnswerIndex: int(q.CorrectAnswerIndex),
		}
	}
	return quiz
}

func toResources(in []mcpResourceArg) []models.Resource {
	resources := make([]models.Resource, len(in))
	for i, r := range in {
		//nolint:exhaustruct //ID/LearningPathID assigned by the repository
		resources[i] = models.Resource{Text: r.Text, SortOrder: i}
	}
	return resources
}

// parseOptionalUUID treats an absent or malformed id as no link; the service
// validates well-formed ones.
func parseOptionalUUID(s *string) *uuid.UUID {
	if s == nil || *s == "" {
		return nil
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return nil
	}
	return &id
}

func (h *tools) mcpListPaths(
	ctx context.Context, args mcpListPathsArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	list, hasMore, err := h.paths.List(ctx, uid, args.Limit, args.Offset)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	return &learningpathsv1.ListLearningPathsResponse{
		LearningPaths: protomap.LearningPaths(list),
		HasMore:       hasMore,
	}, nil
}

func (h *tools) mcpGetPath(
	ctx context.Context, args mcpPathIDArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(args.ID, "learning path ID")
	if err != nil {
		return nil, err
	}
	lp, err := h.paths.Get(ctx, id, uid)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	return &learningpathsv1.GetLearningPathResponse{
		LearningPath: protomap.LearningPath(lp),
	}, nil
}

func (h *tools) mcpGetTodoistStatus(
	ctx context.Context, args mcpPathIDArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(args.ID, "learning path ID")
	if err != nil {
		return nil, err
	}
	state, err := h.todoist.SyncState(ctx, uid, id)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	return protomap.TodoistSyncState(state), nil
}

func (h *tools) mcpGetProgress(
	ctx context.Context, args mcpPathIDArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(args.ID, "learning path ID")
	if err != nil {
		return nil, err
	}
	progress, err := h.paths.Progress(ctx, id, uid)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	return protomap.Progress(progress), nil
}

func (h *tools) mcpCreatePath(
	ctx context.Context, args mcpCreatePathArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	//nolint:exhaustruct //ID/UserID/timestamps assigned by the service/repository
	lp := models.LearningPath{
		Title:             args.Title,
		Goal:              args.Goal,
		Routine:           args.Routine,
		Modules:           toModules(args.Modules),
		Resources:         toResources(args.Resources),
		ReminderSchedules: args.ReminderSchedules,
	}
	created, err := h.paths.Create(ctx, uid, lp)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	return &learningpathsv1.CreateLearningPathResponse{
		LearningPath: protomap.LearningPath(created),
	}, nil
}

func (h *tools) mcpUpdatePath(
	ctx context.Context, args mcpUpdatePathArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(args.ID, "learning path ID")
	if err != nil {
		return nil, err
	}
	//nolint:exhaustruct //UserID/timestamps assigned by the service/repository
	lp := models.LearningPath{
		ID:                id,
		Title:             args.Title,
		Goal:              args.Goal,
		Routine:           args.Routine,
		Modules:           toModules(args.Modules),
		Resources:         toResources(args.Resources),
		ReminderSchedules: args.ReminderSchedules,
	}
	if err = h.paths.Update(ctx, uid, lp); err != nil {
		return nil, connecttools.MapError(err)
	}
	updated, err := h.paths.Get(ctx, id, uid)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	return &learningpathsv1.UpdateLearningPathResponse{
		LearningPath: protomap.LearningPath(updated),
	}, nil
}

func (h *tools) mcpRecordProgress(
	ctx context.Context, args mcpRecordProgressArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	itemID, err := parseID(args.ItemID, "item ID")
	if err != nil {
		return nil, err
	}
	if err = h.paths.RecordItemProgress(ctx, uid, itemID, args.Completed); err != nil {
		return nil, connecttools.MapError(err)
	}
	return &learningpathsv1.RecordItemProgressResponse{}, nil
}
