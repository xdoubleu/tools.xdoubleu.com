package connect

import (
	"context"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
	"tools.xdoubleu.com/internal/platform/factory/internal/services"
)

type repoConnectHandler struct {
	app *Deps
}

var _ factoryv1connect.RepoServiceHandler = (*repoConnectHandler)(nil)

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

// harnessFromProto reads unspecified as Claude Code, what a runner that does not
// report one ran; an unknown value maps to one the service rejects.
func harnessFromProto(h factoryv1.Harness) models.Harness {
	switch h {
	case factoryv1.Harness_HARNESS_CLAUDE_CODE, factoryv1.Harness_HARNESS_UNSPECIFIED:
		return models.HarnessClaudeCode
	case factoryv1.Harness_HARNESS_OPENCODE:
		return models.HarnessOpenCode
	}
	return models.HarnessInvalid
}

func protoRepo(r *models.Repo) *factoryv1.Repo {
	return &factoryv1.Repo{
		Id:                r.ID.String(),
		Owner:             r.Owner,
		Name:              r.Name,
		DefaultBranch:     r.DefaultBranch,
		AutoPickup:        r.AutoPickup,
		SentryProject:     r.SentryProject,
		RedPrLabel:        r.RedPRLabel,
		RedPrBranchPrefix: r.RedPRBranchPrefix,
		//nolint:gosec // bounded to 1..1440 by the service and the column
		TimeLimitMinutes: int32(r.TimeLimitMinutes),
	}
}

func repoInput(r *factoryv1.Repo) services.RepoInput {
	return services.RepoInput{
		Owner:             r.GetOwner(),
		Name:              r.GetName(),
		DefaultBranch:     r.GetDefaultBranch(),
		AutoPickup:        r.GetAutoPickup(),
		SentryProject:     r.GetSentryProject(),
		RedPRLabel:        r.GetRedPrLabel(),
		RedPRBranchPrefix: r.GetRedPrBranchPrefix(),
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
	repos, err := h.app.Services.Repos.List(ctx)
	if err != nil {
		return nil, MapError(err)
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
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	repo, err := h.app.Services.Repos.Create(ctx, actor, repoInput(req.GetRepo()))
	if err != nil {
		return nil, MapError(err)
	}
	return &factoryv1.CreateRepoResponse{Repo: protoRepo(repo)}, nil
}

func (h *repoConnectHandler) UpdateRepo(
	ctx context.Context,
	req *factoryv1.UpdateRepoRequest,
) (*factoryv1.UpdateRepoResponse, error) {
	actor, err := actorFrom(ctx)
	if err != nil {
		return nil, err
	}
	id, err := ParseID(req.GetRepo().GetId())
	if err != nil {
		return nil, err
	}
	repo, err := h.app.Services.Repos.Update(ctx, actor, id, repoInput(req.GetRepo()))
	if err != nil {
		return nil, MapError(err)
	}
	return &factoryv1.UpdateRepoResponse{Repo: protoRepo(repo)}, nil
}
