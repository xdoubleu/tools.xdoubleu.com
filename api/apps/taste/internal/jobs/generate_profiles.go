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
	Due(ctx context.Context, userIDs []string) ([]string, error)
	Generate(ctx context.Context, userID string, include func(appName string) bool) error
}

type pickNotifier interface {
	NotifyNewPicks(ctx context.Context, userID, email string)
}

// SharedLister returns the subject of every family with a shared list.
type SharedLister func(ctx context.Context) ([]string, error)

// SharedApp is the only app a shared profile reads.
const SharedApp = "movies"

// GenerateProfilesJob writes the profiles of users with taste access that
// were requested or have gone stale, then their picks from the new profile,
// and emails them when picks were written. It also retries picks that never
// finished, from the stored profile.
type GenerateProfilesJob struct {
	users    userLister
	profiles profileGenerator
	picks    recommender
	notify   pickNotifier
	appName  string
	shared   SharedLister
}

func NewGenerateProfilesJob(
	users userLister,
	profiles profileGenerator,
	picks recommender,
	notify pickNotifier,
	appName string,
) GenerateProfilesJob {
	return GenerateProfilesJob{
		users: users, profiles: profiles, picks: picks, notify: notify,
		appName: appName, shared: nil,
	}
}

// WithShared also writes the profiles of the subjects lister returns, from
// the shared app only and without emailing anyone.
func (j GenerateProfilesJob) WithShared(lister SharedLister) GenerateProfilesJob {
	j.shared = lister
	return j
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

	sharedIDs, err := j.sharedSubjects(ctx)
	if err != nil {
		return err
	}
	ids = append(ids, sharedIDs...)

	due, err := j.profiles.Due(ctx, ids)
	if err != nil {
		return err
	}
	var errs []error
	for _, id := range due {
		errs = append(errs, j.generate(ctx, logger, id, byID, true))
	}

	// A picks step that never finished leaves a fresh profile that isn't due.
	rest := slices.DeleteFunc(slices.Clone(ids), func(id string) bool {
		return slices.Contains(due, id)
	})
	picksDue, err := j.picks.Due(ctx, rest)
	if err != nil {
		return errors.Join(append(errs, err)...)
	}
	for _, id := range picksDue {
		errs = append(errs, j.generate(ctx, logger, id, byID, false))
	}
	return errors.Join(errs...)
}

// generate writes id's picks, after their profile when withProfile, and emails
// a user when picks were written.
func (j GenerateProfilesJob) generate(
	ctx context.Context,
	logger *slog.Logger,
	id string,
	byID map[string]sharedmodels.User,
	withProfile bool,
) error {
	user, isUser := byID[id]
	include := CanAccess(user)
	if !isUser {
		include = func(appName string) bool { return appName == SharedApp }
	}
	var err error
	if withProfile {
		err = j.profiles.Generate(ctx, id, include)
	}
	if err == nil {
		err = j.picks.Generate(ctx, id, include)
	}
	if err == nil && isUser {
		j.notify.NotifyNewPicks(ctx, id, user.Email)
	}
	if err != nil {
		logger.WarnContext(ctx, "taste generation failed",
			slog.String("user_id", id), slog.Any("error", err))
		return fmt.Errorf("user %s: %w", id, err)
	}
	return nil
}

func (j GenerateProfilesJob) sharedSubjects(ctx context.Context) ([]string, error) {
	if j.shared == nil {
		return nil, nil
	}
	return j.shared(ctx)
}

// CanAccess reports which apps u may read, mirroring auth's AppAccess gate.
func CanAccess(u sharedmodels.User) func(appName string) bool {
	return func(appName string) bool {
		return u.Role == sharedmodels.RoleAdmin || slices.Contains(u.AppAccess, appName)
	}
}
