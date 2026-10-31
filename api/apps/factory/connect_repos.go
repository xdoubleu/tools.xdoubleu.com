package factory

import (
	"context"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/apps/factory/internal/models"
	"tools.xdoubleu.com/apps/factory/internal/services"
	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
)

type repoConnectHandler struct {
	app *Factory
}

var _ factoryv1connect.RepoServiceHandler = (*repoConnectHandler)(nil)

// writerFrom is actorFrom for changes to a repo's settings: its setup commands
// and instructions drive what the sandbox runs, so only a browser session may
// set them, never a bearer token.
func writerFrom(ctx context.Context) error {
	actor, err := actorFrom(ctx)
	if err != nil {
		return err
	}
	if !actor.Browser {
		return connect.NewError(
			connect.CodePermissionDenied,
			"repo settings can only be changed in a browser session",
		)
	}
	return nil
}

func harnessToProto(h models.Harness) factoryv1.Harness {
	switch h {
	case models.HarnessClaudeCode:
		return factoryv1.Harness_HARNESS_CLAUDE_CODE
	case models.HarnessOpenCode:
		return factoryv1.Harness_HARNESS_OPENCODE
	case models.HarnessInvalid:
		return factoryv1.Harness_HARNESS_UNSPECIFIED
	}
	return factoryv1.Harness_HARNESS_UNSPECIFIED
}

// harnessFromProto maps unspecified to "", which the service defaults, and an
// unknown value to one the service rejects.
func harnessFromProto(h factoryv1.Harness) models.Harness {
	switch h {
	case factoryv1.Harness_HARNESS_CLAUDE_CODE:
		return models.HarnessClaudeCode
	case factoryv1.Harness_HARNESS_OPENCODE:
		return models.HarnessOpenCode
	case factoryv1.Harness_HARNESS_UNSPECIFIED:
		return ""
	}
	return models.HarnessInvalid
}

func protoRepo(r *models.Repo) *factoryv1.Repo {
	return &factoryv1.Repo{
		Id:                r.ID.String(),
		Owner:             r.Owner,
		Name:              r.Name,
		DefaultBranch:     r.DefaultBranch,
		DefaultHarness:    harnessToProto(r.DefaultHarness),
		SetupCommands:     r.SetupCommands,
		AgentInstructions: r.AgentInstructions,
		//nolint:gosec // bounded to 1..1440 by the service and the column
		TimeLimitMinutes: int32(r.TimeLimitMinutes),
	}
}

func repoInput(r *factoryv1.Repo) services.RepoInput {
	return services.RepoInput{
		Owner:             r.GetOwner(),
		Name:              r.GetName(),
		DefaultBranch:     r.GetDefaultBranch(),
		DefaultHarness:    harnessFromProto(r.GetDefaultHarness()),
		SetupCommands:     r.GetSetupCommands(),
		AgentInstructions: r.GetAgentInstructions(),
		TimeLimitMinutes:  int(r.GetTimeLimitMinutes()),
	}
}

func (h *repoConnectHandler) ListRepos(
	ctx context.Context,
	_ *factoryv1.ListReposRequest,
) (*factoryv1.ListReposResponse, error) {
	if _, err := actorFrom(ctx); err != nil {
		return nil, err
	}
	repos, err := h.app.services.Repos.List(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*factoryv1.Repo, len(repos))
	for i := range repos {
		out[i] = protoRepo(&repos[i])
	}
	return &factoryv1.ListReposResponse{Repos: out}, nil
}

func (h *repoConnectHandler) CreateRepo(
	ctx context.Context,
	req *factoryv1.CreateRepoRequest,
) (*factoryv1.CreateRepoResponse, error) {
	if err := writerFrom(ctx); err != nil {
		return nil, err
	}
	repo, err := h.app.services.Repos.Create(ctx, repoInput(req.GetRepo()))
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.CreateRepoResponse{Repo: protoRepo(repo)}, nil
}

func (h *repoConnectHandler) UpdateRepo(
	ctx context.Context,
	req *factoryv1.UpdateRepoRequest,
) (*factoryv1.UpdateRepoResponse, error) {
	if err := writerFrom(ctx); err != nil {
		return nil, err
	}
	id, err := parseID(req.GetRepo().GetId())
	if err != nil {
		return nil, err
	}
	repo, err := h.app.services.Repos.Update(ctx, id, repoInput(req.GetRepo()))
	if err != nil {
		return nil, mapError(err)
	}
	return &factoryv1.UpdateRepoResponse{Repo: protoRepo(repo)}, nil
}
