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
	// deployBudget caps the GitHub lookups of one board load; deployConcurrency
	// is how many run at once.
	deployBudget      = 2 * time.Second
	deployConcurrency = 8
	// devRelease is what a build without a deploy reports as its release.
	devRelease = "dev"
)

// Why a merged task's deploy state is unknown, shown on the board.
const (
	ReasonNoMergeCommit = "no merge commit recorded"
	ReasonNoGitHub      = "GitHub App not configured"
	ReasonNoRelease     = "running release unknown (dev build)"
	ReasonCompareFailed = "GitHub compare failed"
)

type deployKey struct{ repo, sha string }

type deployAnswer struct {
	state  models.DeployState
	reason string
	at     time.Time
}

// deployCache remembers answers; a live one is final, anything else expires.
type deployCache struct {
	mu      sync.Mutex
	answers map[deployKey]deployAnswer
}

func newDeployCache() *deployCache {
	return &deployCache{mu: sync.Mutex{}, answers: map[deployKey]deployAnswer{}}
}

func (c *deployCache) get(k deployKey, now time.Time) (deployAnswer, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	a, ok := c.answers[k]
	if !ok || (a.state != models.DeployLive && now.Sub(a.at) > deployRetry) {
		return deployAnswer{state: "", reason: "", at: time.Time{}}, false
	}
	return a, true
}

func (c *deployCache) put(k deployKey, a deployAnswer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.answers[k] = a
}

// FillDeploy sets Deploy on each task that merged recently from whether the
// running release contains its merge commit. When it can't tell (no GitHub App, a
// dev build, no recorded merge commit, GitHub failing or too slow) the state is
// unknown with a reason: a board must load without it. The lookups run
// concurrently under deployBudget.
func (s *JobService) FillDeploy(ctx context.Context, tasks []models.Task) {
	ctx, cancel := context.WithTimeout(ctx, deployBudget)
	defer cancel()
	var (
		repos map[uuid.UUID]models.Repo
		wg    sync.WaitGroup
		sem   = make(chan struct{}, deployConcurrency)
	)
	defer wg.Wait()
	now := s.now()
	for i := range tasks {
		t := &tasks[i]
		if reason := unknownReason(s, t, now); reason != "" {
			if reason != reasonSilent {
				t.Deploy, t.DeployReason = models.DeployUnknown, reason
			}
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
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			a := s.deployState(ctx, repo, t.MergeSHA, now)
			t.Deploy, t.DeployReason = a.state, a.reason
		}()
	}
}

// reasonSilent marks a task that has no deploy state and nothing to say about it.
const reasonSilent = "-"

// unknownReason is why a task's deploy state can't be told without asking
// GitHub, "" when it can be asked.
func unknownReason(s *JobService, t *models.Task, now time.Time) string {
	switch {
	case now.Sub(t.UpdatedAt) > deployWindow,
		t.Status != models.StatusTesting && t.Status != models.StatusDone:
		return reasonSilent
	case t.MergeSHA == "" && t.Status == models.StatusTesting:
		// Only a task in Testing is known to have arrived through a merge.
		return ReasonNoMergeCommit
	case t.MergeSHA == "":
		return reasonSilent
	case s.github == nil:
		return ReasonNoGitHub
	case s.release == "" || s.release == devRelease:
		return ReasonNoRelease
	}
	return ""
}

func (s *JobService) deployState(
	ctx context.Context,
	repo models.Repo,
	sha string,
	now time.Time,
) deployAnswer {
	key := deployKey{repo: repo.Owner + "/" + repo.Name, sha: sha}
	if a, ok := s.deploys.get(key, now); ok {
		return a
	}
	live, err := s.github.Deployed(ctx, repo.Owner, repo.Name, sha, s.release)
	a := deployAnswer{state: models.DeployPending, reason: "", at: now}
	switch {
	case err != nil:
		a.state, a.reason = models.DeployUnknown, ReasonCompareFailed
		if ctx.Err() != nil {
			return a // cut off by the deadline: not GitHub's answer, don't cache
		}
	case live:
		a.state = models.DeployLive
	}
	s.deploys.put(key, a)
	return a
}
