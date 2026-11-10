package factory

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"tools.xdoubleu.com/apps/factory/internal/models"
	"tools.xdoubleu.com/apps/factory/internal/services"
	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/mcptools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// mcpAppName is the app whose grant the factory_ tools need.
const mcpAppName = "factory"

const statusNames = "backlog, ready, in_progress, in_review, needs_you, testing or done"

//nolint:lll // struct tags can't wrap
type mcpListTasksArgs struct {
	Status string `json:"status,omitempty"  jsonschema:"only tasks in this column: backlog, ready, in_progress, in_review, needs_you, testing or done"`
	RepoID string `json:"repo_id,omitempty" jsonschema:"only tasks of this repo id, from factory_list_repos"`
	Label  string `json:"label,omitempty"   jsonschema:"only tasks with this label, ignoring case"`
	Query  string `json:"query,omitempty"   jsonschema:"only tasks whose title or body contains this text, ignoring case"`
}

type mcpTaskArgs struct {
	ID string `json:"id" jsonschema:"task id, such as T-142"`
}

//nolint:lll // struct tags can't wrap
type mcpCreateTaskArgs struct {
	RepoID   string   `json:"repo_id"            jsonschema:"repo id, from factory_list_repos"`
	Title    string   `json:"title"              jsonschema:"short task title"`
	Body     string   `json:"body,omitempty"     jsonschema:"what to do, in markdown"`
	Priority string   `json:"priority,omitempty" jsonschema:"p0, p1 or p2; default p2"`
	Labels   []string `json:"labels,omitempty"   jsonschema:"labels, at most 10"`
}

type mcpCommentArgs struct {
	TaskID string `json:"task_id" jsonschema:"task id, such as T-142"`
	Body   string `json:"body"    jsonschema:"comment text"`
}

//nolint:lll // struct tags can't wrap
type mcpMoveArgs struct {
	ID     string `json:"id"     jsonschema:"task id, such as T-142"`
	Status string `json:"status" jsonschema:"target column: backlog, in_progress, in_review, needs_you, testing or done; ready is refused"`
}

//nolint:lll // struct tags can't wrap
type mcpUpdateArgs struct {
	ID           string   `json:"id"                      jsonschema:"task id, such as T-142"`
	Title        string   `json:"title,omitempty"         jsonschema:"new title; omit to keep it"`
	Body         string   `json:"body,omitempty"          jsonschema:"new body in markdown, replacing the old one; omit to keep it"`
	Priority     string   `json:"priority,omitempty"      jsonschema:"p0, p1 or p2; omit to keep it"`
	AddLabels    []string `json:"add_labels,omitempty"    jsonschema:"labels to add"`
	RemoveLabels []string `json:"remove_labels,omitempty" jsonschema:"labels to remove, ignoring case"`
}

//nolint:lll // struct tags can't wrap
type mcpMoveManyArgs struct {
	IDs    []string `json:"ids"    jsonschema:"task ids such as T-142, at most 200"`
	Status string   `json:"status" jsonschema:"target column: backlog, in_progress, in_review, needs_you, testing or done; ready is refused"`
}

// maxBulkMove bounds one factory_move_tasks call.
const maxBulkMove = 200

// RegisterMCPTools exposes the board to agent accounts (docs/adr-0040). The
// tools act as the agent; moving a task into Ready is refused by the service,
// which only lets a human in a browser do that.
func (a *Factory) RegisterMCPTools(srv *mcp.Server) {
	t := &mcpTools{app: a}

	mcptools.AddAgentReadTool(srv, mcpAppName, "factory_list_repos",
		"The repos the factory works on: id, owner, name and default branch. "+
			"Setup commands and instructions are withheld. "+
			"Use the id when creating a task.", t.listRepos)
	mcptools.AddAgentReadTool(srv, mcpAppName, "factory_list_tasks",
		"Tasks on the board, most recently updated first, without their bodies. "+
			"Filter by status (one of "+statusNames+"), repo_id, label, or query, "+
			"which searches titles and bodies.", t.listTasks)
	mcptools.AddAgentReadTool(srv, mcpAppName, "factory_get_task",
		"A task with its body and comments.", t.getTask)
	mcptools.AddAgentWriteTool(srv, mcpAppName, "factory_create_task",
		"Create a task for a repo. It always starts in Backlog; only a person "+
			"can mark it Ready.", t.createTask)
	mcptools.AddAgentWriteTool(srv, mcpAppName, "factory_comment_task",
		"Add a comment to a task, as this agent.", t.commentTask)
	mcptools.AddAgentWriteTool(srv, mcpAppName, "factory_move_task",
		"Move a task to another column ("+statusNames+"). Moving into ready is "+
			"refused: only a person in a browser can do that.", t.moveTask)
	mcptools.AddAgentWriteTool(srv, mcpAppName, "factory_update_task",
		"Change a task's title, body, priority or labels (add_labels and "+
			"remove_labels). Fields left out stay as they are. It never changes "+
			"the column.", t.updateTask)
	mcptools.AddAgentWriteTool(srv, mcpAppName, "factory_move_tasks",
		"Move many tasks to one column ("+statusNames+") and get back only the "+
			"ids moved and the ones that failed. Moving into ready is refused.",
		t.moveTasks)
}

type mcpTools struct {
	app *Factory
}

// agentActor is the calling agent account as a board actor. It is never a
// browser or an admin, so it can't pass the Ready guard.
func agentActor(ctx context.Context) (models.Actor, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil || user.Role != sharedmodels.RoleAgent {
		return models.Actor{}, errors.New("this tool is for agent accounts only")
	}
	return models.Actor{
		Kind:    models.ActorAgent,
		ID:      user.ID,
		Browser: false,
		Admin:   false,
	}, nil
}

// errTaskNotFound is what a job's account hears for any task but its own: the
// same as for one that doesn't exist, so it can't tell them apart.
var errTaskNotFound = connecttools.MapError(database.ErrResourceNotFound)

// scopedTask is the one task the calling account is confined to: a job's own
// account. The bool is false for an account made by a person.
func (t *mcpTools) scopedTask(ctx context.Context) (uuid.UUID, bool, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return uuid.Nil, false, errors.New("this tool is for agent accounts only")
	}
	userID, err := uuid.Parse(user.ID)
	if err != nil {
		return uuid.Nil, false, errors.New("this tool is for agent accounts only")
	}
	id, err := t.app.services.Jobs.TaskScope(ctx, userID)
	if err != nil {
		return uuid.Nil, false, connecttools.MapError(err)
	}
	if id == nil {
		return uuid.Nil, false, nil
	}
	return *id, true, nil
}

// onlyOwnTask refuses a task other than the scoped one, if there is a scope.
func (t *mcpTools) onlyOwnTask(ctx context.Context, id uuid.UUID) error {
	own, scoped, err := t.scopedTask(ctx)
	if err != nil {
		return err
	}
	if scoped && id != own {
		return errTaskNotFound
	}
	return nil
}

func (t *mcpTools) listRepos(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	repos, err := t.app.services.Repos.List(ctx)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	out := make([]*factoryv1.Repo, len(repos))
	for i := range repos {
		// Setup commands and instructions are for the sandbox, not the agent.
		out[i] = &factoryv1.Repo{
			Id:            repos[i].ID.String(),
			Owner:         repos[i].Owner,
			Name:          repos[i].Name,
			DefaultBranch: repos[i].DefaultBranch,
		}
	}
	return &factoryv1.ListReposResponse{Repos: out}, nil
}

func (t *mcpTools) listTasks(
	ctx context.Context, args mcpListTasksArgs,
) (proto.Message, error) {
	if args.Status != "" && !models.Status(args.Status).Valid() {
		return nil, errors.New("status must be one of " + statusNames)
	}
	var repoID uuid.UUID
	if args.RepoID != "" {
		var err error
		if repoID, err = parseID(args.RepoID); err != nil {
			return nil, err
		}
	}
	own, scoped, err := t.scopedTask(ctx)
	if err != nil {
		return nil, err
	}
	tasks, err := t.app.services.Tasks.List(ctx, args.Query)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	out := []*factoryv1.Task{}
	for i := range tasks {
		if scoped && tasks[i].ID != own {
			continue
		}
		if args.Status != "" && string(tasks[i].Status) != args.Status {
			continue
		}
		if args.RepoID != "" && tasks[i].RepoID != repoID {
			continue
		}
		if !matchesLabel(&tasks[i], args.Label) {
			continue
		}
		out = append(out, protoTask(&tasks[i]))
	}
	return &factoryv1.ListTasksResponse{Tasks: out}, nil
}

// matchesLabel keeps every task when label is empty.
func matchesLabel(task *models.Task, label string) bool {
	if label == "" {
		return true
	}
	return slices.ContainsFunc(task.Labels, func(l string) bool {
		return strings.EqualFold(l, label)
	})
}

func (t *mcpTools) getTask(
	ctx context.Context, args mcpTaskArgs,
) (proto.Message, error) {
	id, err := t.app.taskID(ctx, args.ID)
	if err != nil {
		return nil, err
	}
	if err = t.onlyOwnTask(ctx, id); err != nil {
		return nil, err
	}
	task, comments, err := t.app.services.Tasks.Get(ctx, id)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	out := make([]*factoryv1.Comment, len(comments))
	for i := range comments {
		out[i] = protoComment(&comments[i])
	}
	return &factoryv1.GetTaskResponse{Task: protoTask(task), Comments: out}, nil
}

func (t *mcpTools) createTask(
	ctx context.Context, args mcpCreateTaskArgs,
) (proto.Message, error) {
	actor, err := agentActor(ctx)
	if err != nil {
		return nil, err
	}
	repoID, err := parseID(args.RepoID)
	if err != nil {
		return nil, err
	}
	task, err := t.app.services.Tasks.Create(ctx, actor, repoID, services.TaskInput{
		Title:    args.Title,
		Body:     args.Body,
		Priority: models.Priority(args.Priority),
		Labels:   args.Labels,
	})
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	return &factoryv1.CreateTaskResponse{Task: protoTask(task)}, nil
}

func (t *mcpTools) commentTask(
	ctx context.Context, args mcpCommentArgs,
) (proto.Message, error) {
	actor, err := agentActor(ctx)
	if err != nil {
		return nil, err
	}
	id, err := t.app.taskID(ctx, args.TaskID)
	if err != nil {
		return nil, err
	}
	if err = t.onlyOwnTask(ctx, id); err != nil {
		return nil, err
	}
	comment, err := t.app.services.Tasks.Comment(ctx, actor, id, args.Body)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	return &factoryv1.AddCommentResponse{Comment: protoComment(comment)}, nil
}

func (t *mcpTools) moveTask(
	ctx context.Context, args mcpMoveArgs,
) (proto.Message, error) {
	actor, err := agentActor(ctx)
	if err != nil {
		return nil, err
	}
	id, err := t.app.taskID(ctx, args.ID)
	if err != nil {
		return nil, err
	}
	_, scoped, err := t.scopedTask(ctx)
	if err != nil {
		return nil, err
	}
	if scoped {
		return nil, errors.New(
			"a job can't move tasks: the factory moves its task when the job ends")
	}
	task, err := t.app.services.Tasks.Move(ctx, actor, id, models.Status(args.Status))
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	return &factoryv1.MoveTaskResponse{Task: protoTask(task)}, nil
}

func (t *mcpTools) updateTask(
	ctx context.Context, args mcpUpdateArgs,
) (proto.Message, error) {
	actor, err := agentActor(ctx)
	if err != nil {
		return nil, err
	}
	if err = t.refuseScoped(ctx, "edit tasks"); err != nil {
		return nil, err
	}
	id, err := t.app.taskID(ctx, args.ID)
	if err != nil {
		return nil, err
	}
	current, _, err := t.app.services.Tasks.Get(ctx, id)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	// A person approved this title and body when moving it to Ready.
	if current.Status == models.StatusReady && (args.Title != "" || args.Body != "") {
		return nil, errors.New("the title and body of a Ready task can't be changed")
	}
	in := services.TaskInput{
		Title:    current.Title,
		Body:     current.Body,
		Priority: current.Priority,
		Labels:   mergeLabels(current.Labels, args.AddLabels, args.RemoveLabels),
	}
	if args.Title != "" {
		in.Title = args.Title
	}
	if args.Body != "" {
		in.Body = args.Body
	}
	if args.Priority != "" {
		in.Priority = models.Priority(args.Priority)
	}
	task, err := t.app.services.Tasks.Update(ctx, actor, id, in)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	return &factoryv1.UpdateTaskResponse{Task: protoTask(task)}, nil
}

// mergeLabels applies the removals (ignoring case) and then the additions,
// keeping order and dropping duplicates.
func mergeLabels(current, add, remove []string) []string {
	out := []string{}
	for _, l := range current {
		if !containsFold(remove, l) {
			out = append(out, l)
		}
	}
	for _, l := range add {
		if !containsFold(out, l) {
			out = append(out, l)
		}
	}
	return out
}

func containsFold(list []string, label string) bool {
	return slices.ContainsFunc(list, func(l string) bool {
		return strings.EqualFold(l, label)
	})
}

// refuseScoped stops a job's own account from doing what only a board-wide
// account may.
func (t *mcpTools) refuseScoped(ctx context.Context, what string) error {
	_, scoped, err := t.scopedTask(ctx)
	if err != nil {
		return err
	}
	if scoped {
		return errors.New("a job can't " + what)
	}
	return nil
}

func (t *mcpTools) moveTasks(
	ctx context.Context, args mcpMoveManyArgs,
) (proto.Message, error) {
	actor, err := agentActor(ctx)
	if err != nil {
		return nil, err
	}
	if err = t.refuseScoped(ctx, "move tasks"); err != nil {
		return nil, err
	}
	if len(args.IDs) == 0 || len(args.IDs) > maxBulkMove {
		return nil, errors.New("ids must hold between 1 and 200 task ids")
	}
	status := models.Status(args.Status)
	if !status.Valid() {
		return nil, errors.New("status must be one of " + statusNames)
	}
	if status == models.StatusReady {
		return nil, errors.New("only a person in a browser can move a task to ready")
	}
	moved := make([]any, 0, len(args.IDs))
	failed := []any{}
	for _, raw := range slices.Compact(slices.Sorted(slices.Values(args.IDs))) {
		id, perr := t.app.taskID(ctx, raw)
		if perr == nil {
			_, perr = t.app.services.Tasks.Move(ctx, actor, id, status)
		}
		if perr != nil {
			failed = append(failed, map[string]any{"id": raw, "error": perr.Error()})
			continue
		}
		moved = append(moved, raw)
	}
	return structpb.NewStruct(map[string]any{"moved": moved, "failed": failed})
}
