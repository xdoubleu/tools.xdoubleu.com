package services

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

const (
	// deployRetry is how long a "not yet" or failed answer is reused, so a board
	// poll doesn't ask GitHub per task each time.
	deployRetry = 30 * time.Second
	// deployWindow is how long after its last change a merged task still shows a
	// deploy state: older ones are long live, and checking them all would cost a
	// GitHub call each after a restart.
	deployWindow = 7 * 24 * time.Hour
	// devRelease is what a build without a deploy reports as its release.
	devRelease = "dev"
)

type deployKey struct{ repo, sha string }

type deployAnswer struct {
	state models.DeployState
	at    time.Time
}

// deployCache remembers answers; a live one is final, anything else expires.
type deployCache struct {
	mu      sync.Mutex
	answers map[deployKey]deployAnswer
}

func newDeployCache() *deployCache {
	return &deployCache{mu: sync.Mutex{}, answers: map[deployKey]deployAnswer{}}
}

func (c *deployCache) get(k deployKey, now time.Time) (models.DeployState, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	a, ok := c.answers[k]
	if !ok || (a.state != models.DeployLive && now.Sub(a.at) > deployRetry) {
		return "", false
	}
	return a.state, true
}

func (c *deployCache) put(k deployKey, state models.DeployState, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.answers[k] = deployAnswer{state: state, at: now}
}

// FillDeploy sets Deploy on each merged task from whether the running release
// contains its merge commit. It leaves a task unknown without a GitHub App, on a
// dev build, and whenever GitHub can't answer: a board must load without it.
func (s *JobService) FillDeploy(ctx context.Context, tasks []models.Task) {
	if s.github == nil || s.release == "" || s.release == devRelease {
		return
	}
	var repos map[uuid.UUID]models.Repo
	now := s.now()
	for i := range tasks {
		t := &tasks[i]
		if t.MergeSHA == "" || now.Sub(t.UpdatedAt) > deployWindow {
			continue
		}
		if t.Status != models.StatusTesting && t.Status != models.StatusDone {
			continue
		}
		if repos == nil {
			list, err := s.repos.List(ctx)
			if err != nil {
				return
			}
			repos = make(map[uuid.UUID]models.Repo, len(list))
			for _, r := range list {
				repos[r.ID] = r
			}
		}
		repo, ok := repos[t.RepoID]
		if !ok {
			continue
		}
		t.Deploy = s.deployState(ctx, repo, t.MergeSHA, now)
	}
}

func (s *JobService) deployState(
	ctx context.Context,
	repo models.Repo,
	sha string,
	now time.Time,
) models.DeployState {
	key := deployKey{repo: repo.Owner + "/" + repo.Name, sha: sha}
	if state, ok := s.deploys.get(key, now); ok {
		return state
	}
	live, err := s.github.Deployed(ctx, repo.Owner, repo.Name, sha, s.release)
	state := models.DeployUnknown
	switch {
	case err != nil:
	case live:
		state = models.DeployLive
	default:
		state = models.DeployPending
	}
	s.deploys.put(key, state, now)
	return state
}
