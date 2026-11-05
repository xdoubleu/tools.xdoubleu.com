package services

import (
	"context"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/factory/internal/models"
	"tools.xdoubleu.com/apps/factory/internal/repositories"
)

const (
	maxSetupLen        = 10000
	maxInstructionsLen = 20000
	maxBranchLen       = 255
	minTimeLimit       = 1
	maxTimeLimit       = 1440
)

var (
	ownerPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)
	namePattern   = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
	branchPattern = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
)

type RepoService struct {
	repo *repositories.ReposRepository
}

// RepoInput is the editable part of a repo.
type RepoInput struct {
	Owner             string
	Name              string
	DefaultBranch     string
	DefaultHarness    models.Harness
	DefaultModel      string
	SetupCommands     string
	AgentInstructions string
	TimeLimitMinutes  int
}

// validBranch accepts the characters git allows in the common cases, and
// refuses what git rejects as a ref name or could be read as a git option.
func validBranch(b string) bool {
	if len(b) > maxBranchLen ||
		!branchPattern.MatchString(b) ||
		strings.HasPrefix(b, "-") ||
		strings.HasPrefix(b, "/") ||
		strings.HasSuffix(b, "/") ||
		strings.HasSuffix(b, ".") ||
		strings.Contains(b, "..") ||
		strings.Contains(b, "//") {
		return false
	}
	for _, part := range strings.Split(b, "/") {
		if strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return false
		}
	}
	return true
}

// normaliseRepo trims and validates in, defaulting an unset branch to main,
// harness to Claude Code and time limit to 60 minutes.
func normaliseRepo(in RepoInput) (RepoInput, error) {
	in.Owner = strings.TrimSpace(in.Owner)
	in.Name = strings.TrimSpace(in.Name)
	in.DefaultBranch = strings.TrimSpace(in.DefaultBranch)

	switch {
	case !ownerPattern.MatchString(in.Owner):
		return in, badRequest("invalid owner")
	case !namePattern.MatchString(in.Name) || in.Name == "." || in.Name == "..":
		return in, badRequest("invalid repo name")
	}
	if in.DefaultBranch == "" {
		in.DefaultBranch = "main"
	}
	if !validBranch(in.DefaultBranch) {
		return in, badRequest("invalid default branch")
	}
	if in.DefaultHarness == "" {
		in.DefaultHarness = models.HarnessClaudeCode
	}
	in.DefaultModel = strings.TrimSpace(in.DefaultModel)
	if err := checkModel(in.DefaultHarness, in.DefaultModel); err != nil {
		return in, err
	}
	switch {
	case !in.DefaultHarness.Valid():
		return in, badRequest("invalid harness")
	case utf8.RuneCountInString(in.SetupCommands) > maxSetupLen:
		return in, badRequest("setup commands are too long")
	case utf8.RuneCountInString(in.AgentInstructions) > maxInstructionsLen:
		return in, badRequest("agent instructions are too long")
	}
	if in.TimeLimitMinutes == 0 {
		in.TimeLimitMinutes = defaultTimeLimit
	}
	if in.TimeLimitMinutes < minTimeLimit || in.TimeLimitMinutes > maxTimeLimit {
		return in, badRequest("time limit must be 1 to 1440 minutes")
	}
	return in, nil
}

const defaultTimeLimit = 60

func repoFrom(id uuid.UUID, in RepoInput) models.Repo {
	return models.Repo{
		ID:                id,
		Owner:             in.Owner,
		Name:              in.Name,
		DefaultBranch:     in.DefaultBranch,
		DefaultHarness:    in.DefaultHarness,
		DefaultModel:      in.DefaultModel,
		SetupCommands:     in.SetupCommands,
		AgentInstructions: in.AgentInstructions,
		TimeLimitMinutes:  in.TimeLimitMinutes,
		CreatedAt:         time.Time{},
		UpdatedAt:         time.Time{},
	}
}

// List returns every configured repo.
func (s *RepoService) List(ctx context.Context) ([]models.Repo, error) {
	return s.repo.List(ctx)
}

// Create adds a repo; a repeated owner/name is a conflict.
func (s *RepoService) Create(
	ctx context.Context,
	in RepoInput,
) (*models.Repo, error) {
	in, err := normaliseRepo(in)
	if err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, repoFrom(uuid.Nil, in))
}

// Update replaces every setting of a repo. Unlike Create it applies no
// defaults: a field left unset would silently overwrite the stored value.
func (s *RepoService) Update(
	ctx context.Context,
	id uuid.UUID,
	in RepoInput,
) (*models.Repo, error) {
	if strings.TrimSpace(in.DefaultBranch) == "" ||
		in.DefaultHarness == "" ||
		in.TimeLimitMinutes == 0 {
		return nil, badRequest(
			"default branch, harness and time limit are required",
		)
	}
	in, err := normaliseRepo(in)
	if err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, repoFrom(id, in))
}

// modelPattern is an OpenRouter model id, "vendor/model" with an optional tag.
// It goes into the runner's argv, so it can't start with a dash or hold spaces.
var modelPattern = regexp.MustCompile(
	`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._:-]*$`,
)

const maxModelLen = 100

// checkModel enforces that OpenCode has an OpenRouter model and Claude Code none.
func checkModel(harness models.Harness, model string) error {
	switch {
	case harness == models.HarnessOpenCode && model == "":
		return badRequest("OpenCode needs an OpenRouter model, such as vendor/model")
	case harness == models.HarnessOpenCode &&
		(len(model) > maxModelLen || !modelPattern.MatchString(model)):
		return badRequest("the model must look like vendor/model")
	case harness == models.HarnessClaudeCode && model != "":
		return badRequest("Claude Code chooses its own model")
	}
	return nil
}
