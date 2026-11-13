package github

import (
	"context"
	"fmt"
	"strings"
)

// ClaudeBranchPrefix marks agent-opened PR branches.
const ClaudeBranchPrefix = "claude/"

const (
	perPage = 100
	// maxPages bounds a paged listing; GitHub caps check runs at 1000 anyway.
	maxPages = 10
)

// fetchFailingPullRequests returns open PRs with a failing check that carry
// DependenciesLabel or a ClaudeBranchPrefix head; other PRs have a human
// driving them.
func (c *client) fetchFailingPullRequests(
	ctx context.Context, token, repo string,
) ([]PullRequest, error) {
	wires, err := pagedGet(func(page int) ([]prWire, error) {
		var batch []prWire
		endpoint := fmt.Sprintf(
			"%s/repos/%s/pulls?state=open&per_page=%d&page=%d",
			baseURL, repo, perPage, page,
		)
		return batch, c.get(ctx, endpoint, token, &batch)
	})
	if err != nil {
		return nil, err
	}

	prs := make([]PullRequest, 0, len(wires))
	for _, w := range wires {
		pr := PullRequest{
			Number:        w.Number,
			Title:         w.Title,
			URL:           w.HTMLURL,
			Author:        w.User.Login,
			UpdatedAt:     w.UpdatedAt,
			HeadSHA:       w.Head.SHA,
			HeadRef:       w.Head.Ref,
			Labels:        labelNames(w.Labels),
			FailingChecks: nil,
		}
		if !pr.HasLabel(DependenciesLabel) &&
			!strings.HasPrefix(pr.HeadRef, ClaudeBranchPrefix) {
			continue
		}
		pr.FailingChecks, err = c.fetchFailingChecks(ctx, token, repo, pr.HeadSHA)
		if err != nil {
			return nil, err
		}
		if len(pr.FailingChecks) > 0 {
			prs = append(prs, pr)
		}
	}
	return prs, nil
}

func labelNames(labels []labelWire) []string {
	names := make([]string, len(labels))
	for i, l := range labels {
		names[i] = l.Name
	}
	return names
}

func (c *client) fetchFailingChecks(
	ctx context.Context, token, repo, sha string,
) ([]FailingCheck, error) {
	runs, err := pagedGet(func(page int) ([]checkRunWire, error) {
		var wire checkRunsWire
		endpoint := fmt.Sprintf(
			"%s/repos/%s/commits/%s/check-runs?per_page=%d&page=%d",
			baseURL, repo, sha, perPage, page,
		)
		err := c.get(ctx, endpoint, token, &wire)
		return wire.CheckRuns, err
	})
	if err != nil {
		return nil, err
	}

	checks := make([]FailingCheck, 0, len(runs))
	for _, run := range runs {
		if run.Status != statusCompleted || !failingConclusions[run.Conclusion] {
			continue
		}
		checks = append(checks, FailingCheck{
			Name:       run.Name,
			Conclusion: run.Conclusion,
			URL:        run.HTMLURL,
		})
	}
	return checks, nil
}

// pagedGet collects pages from fetch until one comes back short.
func pagedGet[T any](fetch func(page int) ([]T, error)) ([]T, error) {
	var all []T
	for page := 1; page <= maxPages; page++ {
		batch, err := fetch(page)
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) < perPage {
			break
		}
	}
	return all, nil
}
