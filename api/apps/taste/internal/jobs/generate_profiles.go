// Package jobs runs the taste app's LLM generation off the request path
// (docs/adr-0017-long-request-handler-deadlines.md).
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	sharedmodels "tools.xdoubleu.com/internal/models"
)

// GenerateProfilesID is the job's ID, for ForceRun.
const GenerateProfilesID = "taste-generate-profiles"

type userLister interface {
	GetAllUsers(ctx context.Context) ([]sharedmodels.User, error)
}

type profileGenerator interface {
	Configured() bool
	Due(ctx context.Context, userIDs []string) ([]string, error)
	Generate(ctx context.Context, userID string, include func(appName string) bool) error
}

type recommender interface {
	Generate(ctx context.Context, userID string, include func(appName string) bool) error
}

// GenerateProfilesJob writes the profiles of users with taste access that
// were requested or have gone stale, then their picks from the new profile.
type GenerateProfilesJob struct {
	users    userLister
	profiles profileGenerator
	picks    recommender
	appName  string
}

func NewGenerateProfilesJob(
	users userLister,
	profiles profileGenerator,
	picks recommender,
	appName string,
) GenerateProfilesJob {
	return GenerateProfilesJob{
		users: users, profiles: profiles, picks: picks, appName: appName,
	}
}

func (j GenerateProfilesJob) ID() string {
	return GenerateProfilesID
}

// RunEvery catches requests a ForceRun missed while the job was running.
func (j GenerateProfilesJob) RunEvery() time.Duration {
	return time.Hour
}

func (j GenerateProfilesJob) Run(ctx context.Context, logger *slog.Logger) error {
	if !j.profiles.Configured() {
		return nil
	}
	users, err := j.users.GetAllUsers(ctx)
	if err != nil {
		return err
	}
	byID := map[string]sharedmodels.User{}
	for _, u := range users {
		if CanAccess(u)(j.appName) {
			byID[u.ID] = u
		}
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	due, err := j.profiles.Due(ctx, ids)
	if err != nil {
		return err
	}
	var errs []error
	for _, id := range due {
		include := CanAccess(byID[id])
		genErr := j.profiles.Generate(ctx, id, include)
		if genErr == nil {
			genErr = j.picks.Generate(ctx, id, include)
		}
		if genErr != nil {
			logger.WarnContext(ctx, "taste generation failed",
				slog.String("user_id", id), slog.Any("error", genErr))
			errs = append(errs, fmt.Errorf("user %s: %w", id, genErr))
		}
	}
	return errors.Join(errs...)
}

// CanAccess reports which apps u may read, mirroring auth's AppAccess gate.
func CanAccess(u sharedmodels.User) func(appName string) bool {
	return func(appName string) bool {
		return u.Role == sharedmodels.RoleAdmin || slices.Contains(u.AppAccess, appName)
	}
}
