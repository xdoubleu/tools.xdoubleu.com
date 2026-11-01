package factory

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/factory/internal/models"
	"tools.xdoubleu.com/apps/factory/internal/services"
	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	"tools.xdoubleu.com/internal/mcptools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

const statusNames = "backlog, ready, in_progress, in_review, needs_you or done"

//nolint:lll // struct tags can't wrap
type mcpListTasksArgs struct {
	Status string `json:"status,omitempty"  jsonschema:"only tasks in this column: backlog, ready, in_progress, in_review, needs_you or done"`
	RepoID string `json:"repo_id,omitempty" jsonschema:"only tasks of this repo id, from factory_list_repos"`
}

type mcpTaskArgs struct {
	ID string `json:"id" jsonschema:"task id"`
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
	TaskID string `json:"task_id" jsonschema:"task id"`
	Body   string `json:"body"    jsonschema:"comment text"`
}

//nolint:lll // struct tags can't wrap
type mcpMoveArgs struct {
	ID     string `json:"id"     jsonschema:"task id"`
	Status string `json:"status" jsonschema:"target column: backlog, in_progress, in_review, needs_you or done; ready is refused"`
}

// RegisterMCPTools exposes the board to agent accounts (docs/adr-0040). The
// tools act as the agent; moving a task into Ready is refused by the service,
// which only lets a human in a browser do that.
func (a *Factory) RegisterMCPTools(srv *mcp.Server) {
	t := &mcpTools{app: a}

	mcptools.AddAgentReadTool(srv, "factory_list_repos",
		"The repos the factory works on: id, owner, name and default branch. "+
			"Setup commands and instructions are withheld. "+
			"Use the id when creating a task.", t.listRepos)
	mcptools.AddAgentReadTool(srv, "factory_list_tasks",
		"Tasks on the board, most recently updated first, without their bodies. "+
			"Filter by status (one of "+statusNames+") or repo_id.", t.listTasks)
	mcptools.AddAgentReadTool(srv, "factory_get_task",
		"A task with its body and comments.", t.getTask)
	mcptools.AddAgentWriteTool(srv, "factory_create_task",
		"Create a task for a repo. It always starts in Backlog; only a person "+
			"can mark it Ready.", t.createTask)
	mcptools.AddAgentWriteTool(srv, "factory_comment_task",
		"Add a comment to a task, as this agent.", t.commentTask)
	mcptools.AddAgentWriteTool(srv, "factory_move_task",
		"Move a task to another column ("+statusNames+"). Moving into ready is "+
			"refused: only a person in a browser can do that.", t.moveTask)
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
	tasks, err := t.app.services.Tasks.List(ctx)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	out := []*factoryv1.Task{}
	for i := range tasks {
		if args.Status != "" && string(tasks[i].Status) != args.Status {
			continue
		}
		if args.RepoID != "" && tasks[i].RepoID != repoID {
			continue
		}
		out = append(out, protoTask(&tasks[i]))
	}
	return &factoryv1.ListTasksResponse{Tasks: out}, nil
}

func (t *mcpTools) getTask(
	ctx context.Context, args mcpTaskArgs,
) (proto.Message, error) {
	id, err := parseID(args.ID)
	if err != nil {
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
	id, err := parseID(args.TaskID)
	if err != nil {
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
	id, err := parseID(args.ID)
	if err != nil {
		return nil, err
	}
	task, err := t.app.services.Tasks.Move(ctx, actor, id, models.Status(args.Status))
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	return &factoryv1.MoveTaskResponse{Task: protoTask(task)}, nil
}
