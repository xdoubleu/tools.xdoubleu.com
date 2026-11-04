package githubapp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

const (
	// refreshBefore is how long before a read token expires it is replaced.
	refreshBefore = 5 * time.Minute
	// pageSize and maxPages bound what is read of a long list: 500 is far more than
	// any pull request here has.
	pageSize = 100
	maxPages = 5
)

// Pull request, CI and review states, as the factory shows them.
const (
	StateOpen   = "open"
	StateMerged = "merged"
	StateClosed = "closed"

	CIPending = "pending"
	CIPassing = "passing"
	CIFailing = "failing"

	ReviewApproved         = "approved"
	ReviewChangesRequested = "changes_requested"
)

// PullRequestStatus is where a pull request stands. CI and Review are empty when
// there is nothing to report (no checks, no reviews) or once it is no longer open.
type PullRequestStatus struct {
	State  string
	CI     string
	Review string
}

// ErrPullRequestGone is returned when GitHub has no such pull request.
var ErrPullRequestGone = errors.New("github: pull request not found")

// readToken is an installation token that can only read pull requests and their
// checks, remembered until shortly before it expires: polling every few minutes
// must not mint a token per call.
func (c *Client) readToken(ctx context.Context, owner, repo string) (string, error) {
	key := owner + "/" + repo
	c.mu.Lock()
	cached, ok := c.readTokens[key]
	c.mu.Unlock()
	if ok && c.now().Before(cached.ExpiresAt.Add(-refreshBefore)) {
		return cached.Value, nil
	}
	token, err := c.mint(ctx, owner, repo,
		map[string]string{"pull_requests": "read", "checks": "read"})
	if err != nil {
		c.mu.Lock()
		delete(c.readTokens, key)
		c.mu.Unlock()
		return "", err
	}
	c.mu.Lock()
	c.readTokens[key] = token
	c.mu.Unlock()
	return token.Value, nil
}

// PullRequest reads a pull request of owner/repo with a read-only token: whether
// it is open, merged or closed, and for an open one its checks and reviews.
func (c *Client) PullRequest(
	ctx context.Context,
	owner, repo string,
	number int,
) (PullRequestStatus, error) {
	if number < 1 {
		return PullRequestStatus{}, errors.New("github: invalid pull request number")
	}
	token, err := c.readToken(ctx, owner, repo)
	if err != nil {
		return PullRequestStatus{}, err
	}
	base := "/repos/" + owner + "/" + repo
	var pull struct {
		State  string `json:"state"`
		Merged bool   `json:"merged"`
		Head   struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	status, err := c.do(ctx, http.MethodGet, base+"/pulls/"+strconv.Itoa(number),
		token, nil, &pull)
	if status == http.StatusNotFound {
		return PullRequestStatus{}, ErrPullRequestGone
	}
	if err != nil {
		return PullRequestStatus{}, err
	}
	switch {
	case pull.Merged:
		return PullRequestStatus{State: StateMerged, CI: "", Review: ""}, nil
	case pull.State != "open":
		return PullRequestStatus{State: StateClosed, CI: "", Review: ""}, nil
	}

	ci, err := c.checks(ctx, base, pull.Head.SHA, token)
	if err != nil {
		return PullRequestStatus{}, err
	}
	review, err := c.reviews(ctx, base, number, token)
	if err != nil {
		return PullRequestStatus{}, err
	}
	return PullRequestStatus{State: StateOpen, CI: ci, Review: review}, nil
}

// failed reports whether a finished check run's conclusion is a failure.
func failed(conclusion string) bool {
	switch conclusion {
	case "failure", "timed_out", "cancelled", "action_required", "startup_failure",
		"stale":
		return true
	}
	return false
}

// checks rolls the head commit's check runs up: failing if any failed, else
// pending if any hasn't finished, else passing; empty when there are none.
func (c *Client) checks(
	ctx context.Context,
	base, sha, token string,
) (string, error) {
	if sha == "" {
		return "", nil
	}
	type checkRun struct {
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
	}
	var all []checkRun
	for page := 1; page <= maxPages; page++ {
		var runs struct {
			Total     int        `json:"total_count"`
			CheckRuns []checkRun `json:"check_runs"`
		}
		_, err := c.do(ctx, http.MethodGet,
			base+"/commits/"+sha+"/check-runs?per_page=100&page="+strconv.Itoa(page),
			token, nil, &runs)
		if err != nil {
			return "", fmt.Errorf("checks: %w", err)
		}
		all = append(all, runs.CheckRuns...)
		if len(runs.CheckRuns) == 0 || len(all) >= runs.Total {
			break
		}
	}
	if len(all) == 0 {
		return "", nil
	}
	pending := false
	for _, run := range all {
		switch {
		case run.Status != "completed":
			pending = true
		case failed(run.Conclusion):
			return CIFailing, nil
		}
	}
	if pending {
		return CIPending, nil
	}
	return CIPassing, nil
}

// reviews is the pull request's standing review: changes requested if any
// reviewer's latest word is that, else approved if anyone approved, else empty.
// Comments, pending and dismissed reviews don't count.
func (c *Client) reviews(
	ctx context.Context,
	base string,
	number int,
	token string,
) (string, error) {
	type review struct {
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		State string `json:"state"`
	}
	var all []review
	for page := 1; page <= maxPages; page++ {
		var got []review
		_, err := c.do(ctx, http.MethodGet,
			base+"/pulls/"+strconv.Itoa(number)+"/reviews?per_page="+
				strconv.Itoa(pageSize)+"&page="+strconv.Itoa(page),
			token, nil, &got)
		if err != nil {
			return "", fmt.Errorf("reviews: %w", err)
		}
		all = append(all, got...)
		if len(got) < pageSize {
			break
		}
	}
	// GitHub lists a pull request's reviews oldest first.
	latest := map[string]string{}
	for _, r := range all {
		switch r.State {
		case "APPROVED", "CHANGES_REQUESTED":
			latest[r.User.Login] = r.State
		case "DISMISSED":
			delete(latest, r.User.Login)
		}
	}
	approved := false
	for _, state := range latest {
		if state == "CHANGES_REQUESTED" {
			return ReviewChangesRequested, nil
		}
		approved = approved || state == "APPROVED"
	}
	if approved {
		return ReviewApproved, nil
	}
	return "", nil
}
