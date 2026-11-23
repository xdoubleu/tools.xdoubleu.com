package mcp

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
	"tools.xdoubleu.com/internal/platform/factory/internal/services"
	"tools.xdoubleu.com/internal/platform/factory/present"
	"tools.xdoubleu.com/internal/platform/mcptools"
)

// mcpAppName is the app whose grant the factory_ tools need.
const mcpAppName = "factory"

const statusNames = services.StatusNames

//nolint:lll // struct tags can't wrap
type mcpListTasksArgs struct {
	Status string `json:"status,omitempty"  jsonschema:"only tasks in this column: backlog, ready, in_progress, in_review, needs_you, testing or done"`
	RepoID string `json:"repo_id,omitempty" jsonschema:"only tasks of this repo id, from factory_list_repos"`
	Label  string `json:"label,omitempty"   jsonschema:"only tasks with this label, ignoring case"`
	Query  string `json:"query,omitempty"   jsonschema:"only tasks whose title or body contains this text, ignoring case"`
}

//nolint:lll // struct tags can't wrap
type mcpJobArgs struct {
	JobID string `json:"job_id"         jsonschema:"job id, from factory_get_task"`
	Tail  int    `json:"tail,omitempty" jsonschema:"how many of the job's last log events to return; default 50, at most 200"`
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
	Status string `json:"status" jsonschema:"target column: backlog, ready, in_progress, in_review, needs_you, testing or done"`
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
	Status string   `json:"status" jsonschema:"target column: backlog, ready, in_progress, in_review, needs_you, testing or done"`
}

//nolint:lll // struct tags can't wrap
type mcpSetRoutineArgs struct {
	Name            string `json:"name"             jsonschema:"routine name: lower-case letters, digits and dashes; the runner only runs names it knows"`
	RepoID          string `json:"repo_id"          jsonschema:"repo id, from factory_list_repos"`
	Enabled         bool   `json:"enabled"          jsonschema:"whether the schedule queues it; enabling is the approval to run unattended"`
	IntervalMinutes int    `json:"interval_minutes" jsonschema:"minutes between runs, 0 for manual only, at most a month"`
}

type mcpDeleteRoutineArgs struct {
	Name string `json:"name" jsonschema:"routine name, from factory_list_routines"`
}

type mcpRunRoutineArgs struct {
	Name string `json:"name" jsonschema:"routine name, from factory_list_routines"`
}

//nolint:lll // struct tags can't wrap
type mcpChatAskArgs struct {
	Question string   `json:"question,omitempty"    jsonschema:"the question to put to the person; required unless ask_id is set"`
	Options  []string `json:"options,omitempty"     jsonschema:"2 to 4 short options to choose from; two options make a confirmation. The person can also type another answer."`
	Multi    bool     `json:"multi_select,omitempty" jsonschema:"whether the person may choose several options"`
	AskID    int      `json:"ask_id,omitempty"      jsonschema:"the ask_id of a question still waiting, to keep waiting for its answer; leave out for a new question"`
}

// ChatPublisher pushes a chat's new events to the sockets watching it.
type ChatPublisher interface {
	Publish(id uuid.UUID)
}

// ChatAskWait is how long one factory_chat_ask call waits for the answer: well
// within the request's budget, after which the agent asks again with the ask_id.
// It is a variable so tests can shorten it.
//
//nolint:gochecknoglobals // a tunable, set only by tests
var ChatAskWait = 15 * time.Second

// Register exposes the board to agent accounts (docs/adr-0040). The
// tools act as the agent, which may move a task into Ready; the move is
// recorded as a comment.
func Register(srv *mcp.Server, svc *services.Services, chats ChatPublisher) {
	t := &mcpTools{
		svc: svc, present: present.Presenter{Labels: svc.Jobs}, chats: chats,
	}

	mcptools.AddAgentReadTool(srv, mcpAppName, "factory_list_repos",
		"The repos the factory works on: id, owner, name and default branch. "+
			"Setup commands and instructions are withheld. "+
			"Use the id when creating a task.", t.listRepos)
	mcptools.AddAgentReadTool(srv, mcpAppName, "factory_list_tasks",
		"Tasks on the board, most recently updated first, without their bodies. "+
			"Filter by status (one of "+statusNames+"), repo_id, label, or query, "+
			"which searches titles and bodies.", t.listTasks)
	mcptools.AddAgentReadTool(srv, mcpAppName, "factory_get_task",
		"A task with its body, comments and jobs (id, status, harness, outcome: "+
			"how the job ended, or why it failed or was stopped).", t.getTask)
	mcptools.AddAgentReadTool(srv, mcpAppName, "factory_get_job",
		"A job with its outcome and the last events of its harness log, oldest "+
			"first, credential values removed. The events can hold code and file "+
			"contents of the job's repo.", t.getJob)
	mcptools.AddAgentWriteTool(srv, mcpAppName, "factory_create_task",
		"Create a task for a repo. It starts in Backlog, except a task labelled "+
			"bug or factory-retro, which starts in Ready.", t.createTask)
	mcptools.AddAgentWriteTool(srv, mcpAppName, "factory_comment_task",
		"Add a comment to a task, as this agent.", t.commentTask)
	mcptools.AddAgentWriteTool(srv, mcpAppName, "factory_move_task",
		"Move a task to another column ("+statusNames+"). Moving into ready "+
			"re-queues a task you unblocked yourself; the move is recorded "+
			"with your name.", t.moveTask)
	mcptools.AddAgentWriteTool(srv, mcpAppName, "factory_update_task",
		"Change a task's title, body, priority or labels (add_labels and "+
			"remove_labels). Fields left out stay as they are. It never changes "+
			"the column.", t.updateTask)
	mcptools.AddAgentWriteTool(srv, mcpAppName, "factory_move_tasks",
		"Move many tasks to one column ("+statusNames+") and get back only the "+
			"ids moved and the ones that failed.",
		t.moveTasks)
	mcptools.AddAgentReadTool(srv, mcpAppName, "factory_list_routines",
		"Every routine with its repo, enabled flag, interval and newest job.",
		t.listRoutines)
	mcptools.AddAgentWriteTool(srv, mcpAppName, "factory_set_routine",
		"Create or change a routine: repo, enabled flag and interval. The change "+
			"is announced as made by this agent. What a routine does is decided "+
			"by the runner's own list, not here.", t.setRoutine)
	mcptools.AddAgentWriteTool(srv, mcpAppName, "factory_delete_routine",
		"Delete a routine and cancel its queued or running job. Its past jobs "+
			"stay. A job can't delete the routine it runs for.", t.deleteRoutine)
	mcptools.AddAgentWriteTool(srv, mcpAppName, "factory_run_routine",
		"Queue a job for a routine now, even when it is disabled. One already "+
			"queued or running is a conflict.", t.runRoutine)
	mcptools.AddAgentWriteTool(srv, mcpAppName, "factory_chat_ask",
		"Ask the person you are chatting with a question and wait for the "+
			"answer, which shows as a card with your options and a free-text "+
			"\"other\". Only a chat session can call it. Give 2 to 4 options, or "+
			"two for a confirmation. The call waits about 15 seconds; when the "+
			"result says it is still waiting, call factory_chat_ask again with the "+
			"ask_id it returned, and keep doing so until it holds the answer.",
		t.chatAsk)
}

type mcpTools struct {
	svc     *services.Services
	present present.Presenter
	chats   ChatPublisher
}

// agentActor is the calling account as a board actor: an agent, or a person
// signed in over MCP, never a browser session, with their own admin role.
func agentActor(ctx context.Context) (models.Actor, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil || user.Role == sharedmodels.RoleService {
		return models.Actor{}, errors.New("this tool is for people and agent accounts only")
	}
	if user.Role != sharedmodels.RoleAgent {
		return models.Actor{
			Kind:    models.ActorHuman,
			ID:      user.ID,
			Browser: false,
			Admin:   user.Role == sharedmodels.RoleAdmin,
			Via:     "",
		}, nil
	}
	return models.Actor{
		Kind:    models.ActorAgent,
		ID:      user.ID,
		Browser: false,
		Admin:   false,
		Via:     "",
	}, nil
}

func (t *mcpTools) listRepos(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	repos, err := t.svc.Repos.List(ctx)
	if err != nil {
		return nil, present.MapError(err)
	}
	out := make([]*factoryv1.Repo, len(repos))
	for i := range repos {
		out[i] = present.AgentRepo(&repos[i])
	}
	return &factoryv1.ListReposResponse{Repos: out}, nil
}

func (t *mcpTools) listTasks(
	ctx context.Context, args mcpListTasksArgs,
) (proto.Message, error) {
	actor, err := agentActor(ctx)
	if err != nil {
		return nil, err
	}
	filter := models.TaskFilter{
		Query: args.Query, Status: models.Status(args.Status), RepoID: nil,
		Label: args.Label,
	}
	if args.RepoID != "" {
		repoID, idErr := present.ParseID(args.RepoID)
		if idErr != nil {
			return nil, idErr
		}
		filter.RepoID = &repoID
	}
	board, err := t.svc.Board.ListTasks(ctx, actor, filter)
	if err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.ListTasksResponse{
		Tasks: t.present.Tasks(ctx, board.Tasks), PickupPaused: board.PickupPaused,
		ClaudeLimitedUntil: "", QueueStalled: board.QueueStalled,
	}, nil
}

func (t *mcpTools) getTask(
	ctx context.Context, args mcpTaskArgs,
) (proto.Message, error) {
	actor, err := agentActor(ctx)
	if err != nil {
		return nil, err
	}
	detail, err := t.svc.Board.GetTask(ctx, actor, args.ID)
	if err != nil {
		return nil, present.MapError(err)
	}
	return t.present.Detail(ctx, detail), nil
}

func (t *mcpTools) getJob(
	ctx context.Context, args mcpJobArgs,
) (proto.Message, error) {
	actor, err := agentActor(ctx)
	if err != nil {
		return nil, err
	}
	id, err := present.ParseID(args.JobID)
	if err != nil {
		return nil, err
	}
	job, events, err := t.svc.Board.GetJob(ctx, actor, id, args.Tail)
	if err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.GetJobResponse{
		Job:    present.ProtoJob(job),
		Events: present.ProtoJobEvents(events),
	}, nil
}

func (t *mcpTools) createTask(
	ctx context.Context, args mcpCreateTaskArgs,
) (proto.Message, error) {
	actor, err := agentActor(ctx)
	if err != nil {
		return nil, err
	}
	repoID, err := present.ParseID(args.RepoID)
	if err != nil {
		return nil, err
	}
	task, err := t.svc.Board.CreateTask(ctx, actor, repoID, services.TaskInput{
		Title:    args.Title,
		Body:     args.Body,
		Priority: models.Priority(args.Priority),
		Labels:   args.Labels,
	})
	if err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.CreateTaskResponse{Task: t.present.Task(ctx, task)}, nil
}

func (t *mcpTools) commentTask(
	ctx context.Context, args mcpCommentArgs,
) (proto.Message, error) {
	actor, err := agentActor(ctx)
	if err != nil {
		return nil, err
	}
	comment, err := t.svc.Board.AddComment(ctx, actor, args.TaskID, args.Body)
	if err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.AddCommentResponse{
		Comment: t.present.Comment(ctx, comment),
	}, nil
}

func (t *mcpTools) moveTask(
	ctx context.Context, args mcpMoveArgs,
) (proto.Message, error) {
	actor, err := agentActor(ctx)
	if err != nil {
		return nil, err
	}
	task, err := t.svc.Board.MoveTask(
		ctx, actor, args.ID, models.Status(args.Status))
	if err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.MoveTaskResponse{Task: t.present.Task(ctx, task)}, nil
}

func (t *mcpTools) updateTask(
	ctx context.Context, args mcpUpdateArgs,
) (proto.Message, error) {
	actor, err := agentActor(ctx)
	if err != nil {
		return nil, err
	}
	// An omitted field is left as it is.
	patch := services.TaskPatch{
		Title: nil, Body: nil, Priority: nil, Labels: nil,
		AddLabels: args.AddLabels, RemoveLabels: args.RemoveLabels,
	}
	if args.Title != "" {
		patch.Title = &args.Title
	}
	if args.Body != "" {
		patch.Body = &args.Body
	}
	if args.Priority != "" {
		priority := models.Priority(args.Priority)
		patch.Priority = &priority
	}
	task, err := t.svc.Board.UpdateTask(ctx, actor, args.ID, patch)
	if err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.UpdateTaskResponse{Task: t.present.Task(ctx, task)}, nil
}

func (t *mcpTools) moveTasks(
	ctx context.Context, args mcpMoveManyArgs,
) (proto.Message, error) {
	actor, err := agentActor(ctx)
	if err != nil {
		return nil, err
	}
	res, err := t.svc.Board.MoveTasks(
		ctx, actor, args.IDs, models.Status(args.Status))
	if err != nil {
		return nil, present.MapError(err)
	}
	moved := make([]any, len(res.Moved))
	for i, id := range res.Moved {
		moved[i] = id
	}
	failed := make([]any, len(res.Failed))
	for i, f := range res.Failed {
		failed[i] = map[string]any{"id": f.ID, "error": f.Error}
	}
	return structpb.NewStruct(map[string]any{"moved": moved, "failed": failed})
}

func (t *mcpTools) listRoutines(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	list, err := t.svc.Board.ListRoutines(ctx)
	if err != nil {
		return nil, present.MapError(err)
	}
	out := make([]*factoryv1.Routine, len(list))
	for i := range list {
		out[i] = present.ProtoRoutine(&list[i])
	}
	return &factoryv1.ListRoutinesResponse{Routines: out}, nil
}

func (t *mcpTools) setRoutine(
	ctx context.Context, args mcpSetRoutineArgs,
) (proto.Message, error) {
	actor, err := agentActor(ctx)
	if err != nil {
		return nil, err
	}
	repoID, err := present.ParseID(args.RepoID)
	if err != nil {
		return nil, err
	}
	routine, err := t.svc.Board.SaveRoutine(ctx, actor, services.RoutineInput{
		Name: args.Name, RepoID: repoID, Enabled: args.Enabled,
		IntervalMinutes: args.IntervalMinutes,
	})
	if err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.SaveRoutineResponse{Routine: present.ProtoRoutine(routine)}, nil
}

func (t *mcpTools) deleteRoutine(
	ctx context.Context, args mcpDeleteRoutineArgs,
) (proto.Message, error) {
	actor, err := agentActor(ctx)
	if err != nil {
		return nil, err
	}
	if err = t.svc.Board.DeleteRoutine(ctx, actor, args.Name); err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.DeleteRoutineResponse{}, nil
}

func (t *mcpTools) runRoutine(
	ctx context.Context, args mcpRunRoutineArgs,
) (proto.Message, error) {
	actor, err := agentActor(ctx)
	if err != nil {
		return nil, err
	}
	job, err := t.svc.Board.RunRoutine(ctx, actor, args.Name)
	if err != nil {
		return nil, present.MapError(err)
	}
	return &factoryv1.RunRoutineResponse{Job: present.ProtoJob(job)}, nil
}

func (t *mcpTools) chatAsk(
	ctx context.Context, args mcpChatAskArgs,
) (proto.Message, error) {
	actor, err := agentActor(ctx)
	if err != nil {
		return nil, err
	}
	userID, err := uuid.Parse(actor.ID)
	if err != nil || actor.Kind != models.ActorAgent {
		return nil, errors.New("only a chat session can ask the person a question")
	}
	job, err := t.svc.Jobs.JobOf(ctx, userID)
	if err != nil {
		return nil, present.MapError(err)
	}
	if job == nil || !job.IsChat() {
		return nil, errors.New("only a chat session can ask the person a question")
	}
	reply, err := t.svc.Chats.AskPerson(ctx, job.ID, args.AskID,
		services.AskInput{
			Question: args.Question, Options: args.Options, Multi: args.Multi,
		}, ChatAskWait, t.chats.Publish)
	if err != nil {
		return nil, present.MapError(err)
	}
	if !reply.Answered {
		return structpb.NewStruct(map[string]any{
			"status": "waiting",
			"ask_id": reply.AskID,
			"note": "The person has not answered yet. Call factory_chat_ask again with " +
				"this ask_id to keep waiting.",
		})
	}
	return structpb.NewStruct(map[string]any{
		"status": "answered", "ask_id": reply.AskID, "answer": reply.Answer,
	})
}
