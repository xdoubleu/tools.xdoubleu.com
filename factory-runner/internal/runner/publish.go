package runner

import (
	"bytes"
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	githubAPI = "https://api.github.com"
	// publishTimeout bounds the push and the pull request together.
	publishTimeout = 3 * time.Minute
	maxTitleRunes  = 200
	maxBodyRunes   = 60000
	maxGitHubBody  = 1 << 20
	// publishTries is how often a transient failure is tried before giving up.
	publishTries = 3
	// PublishRetry is the default pause between those tries.
	PublishRetry = 2 * time.Second
	zeroWidth    = "\u200b"
)

// PullRequest is what the runner opens for a job's branch.
type PullRequest struct {
	Owner, Repo string
	Head, Base  string
	Title, Body string
}

// GitHubAPI opens pull requests. It is a seam so tests never reach GitHub.
type GitHubAPI interface {
	// OpenPullRequest returns the address of the new pull request, or of the open
	// one the branch already has.
	OpenPullRequest(ctx context.Context, token string, pr PullRequest) (string, error)
}

// GitHubClient is the real GitHubAPI.
type GitHubClient struct {
	BaseURL string
	HTTP    *http.Client
}

// NewGitHubClient talks to api.github.com.
func NewGitHubClient() *GitHubClient {
	return &GitHubClient{BaseURL: githubAPI, HTTP: &http.Client{Timeout: time.Minute}}
}

func (c *GitHubClient) do(
	ctx context.Context,
	method, path, token string,
	in, out any,
) error {
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return err //nolint:wrapcheck // plain data
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return err //nolint:wrapcheck // the caller names it
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err //nolint:wrapcheck // the caller names it
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxGitHubBody))
	if err != nil {
		return err //nolint:wrapcheck // the caller names it
	}
	if resp.StatusCode >= http.StatusMultipleChoices {
		return &githubStatusError{status: resp.StatusCode, body: data}
	}
	if out != nil {
		if err = json.Unmarshal(data, out); err != nil {
			return errors.New("unreadable answer")
		}
	}
	return nil
}

// githubStatusError keeps GitHub's answer for inspection but reports only the
// status: none of GitHub's text belongs in a task comment.
type githubStatusError struct {
	status int
	body   []byte
}

func (e *githubStatusError) Error() string { return fmt.Sprintf("status %d", e.status) }

// OpenPullRequest implements GitHubAPI. A branch that already has an open pull
// request (a second job on the task) gets that one back.
func (c *GitHubClient) OpenPullRequest(
	ctx context.Context,
	token string,
	pr PullRequest,
) (string, error) {
	base := "/repos/" + url.PathEscape(pr.Owner) + "/" + url.PathEscape(pr.Repo) + "/pulls"
	var created struct {
		HTMLURL string `json:"html_url"`
		NodeID  string `json:"node_id"`
	}
	err := c.do(ctx, http.MethodPost, base, token, map[string]any{
		"title": pr.Title, "head": pr.Head, "base": pr.Base, "body": pr.Body,
		"maintainer_can_modify": false,
	}, &created)
	var status *githubStatusError
	if errors.As(err, &status) && status.status == http.StatusUnprocessableEntity &&
		bytes.Contains(status.body, []byte("pull request already exists")) {
		return c.existing(ctx, base, token, pr)
	}
	if err != nil {
		return "", fmt.Errorf("open a pull request: %w", err)
	}
	if created.HTMLURL == "" {
		return "", errors.New("open a pull request: no address in the answer")
	}
	c.armAutoMerge(ctx, token, created.NodeID)
	return created.HTMLURL, nil
}

// armAutoMerge turns on squash auto-merge, so a green ci-pass merges the pull
// request like any other. The pull request is open already, so a refusal (auto-merge
// off for the repo, a missing permission) is logged, not a reason to fail the job.
func (c *GitHubClient) armAutoMerge(ctx context.Context, token, nodeID string) {
	if nodeID == "" {
		return
	}
	var answer struct {
		Errors []json.RawMessage `json:"errors"`
	}
	err := c.do(ctx, http.MethodPost, "/graphql", token, map[string]any{
		"query": "mutation($id: ID!) { enablePullRequestAutoMerge(input: " +
			"{pullRequestId: $id, mergeMethod: SQUASH}) { clientMutationId } }",
		"variables": map[string]any{"id": nodeID},
	}, &answer)
	if err == nil && len(answer.Errors) > 0 {
		err = errors.New("refused")
	}
	if err != nil {
		slog.WarnContext(ctx, "could not arm auto-merge", "error", err)
	}
}

func (c *GitHubClient) existing(
	ctx context.Context,
	base, token string,
	pr PullRequest,
) (string, error) {
	query := url.Values{
		"head": {pr.Owner + ":" + pr.Head}, "state": {"open"}, "per_page": {"1"},
	}
	var found []struct {
		HTMLURL string `json:"html_url"`
		NodeID  string `json:"node_id"`
	}
	path := base + "?" + query.Encode()
	if err := c.do(ctx, http.MethodGet, path, token, nil, &found); err != nil {
		return "", fmt.Errorf("find the open pull request: %w", err)
	}
	if len(found) == 0 || found[0].HTMLURL == "" {
		return "", errors.New("find the open pull request: none found")
	}
	c.armAutoMerge(ctx, token, found[0].NodeID)
	return found[0].HTMLURL, nil
}

var (
	titlePrefix = regexp.MustCompile(`(?i)^\s*(?:#+\s*)?(?:pr\s+title|title)\s*:\s*`)
	// titleLabel is an explicit "PR title:" line, which wins wherever it is.
	titleLabel = regexp.MustCompile(`(?i)^\s*(?:#+\s*)?pr\s+title\s*:\s*\S`)
	// mention is an @name GitHub would notify.
	mention = regexp.MustCompile(`(^|[^\w` + "`" + `@/])@([A-Za-z0-9])`)
	// closing is a keyword GitHub reads as "close this issue on merge", followed by
	// an issue or pull request reference or address.
	closing = regexp.MustCompile(
		`(?i)\b(clos(?:e|es|ed)|fix(?:es|ed)?|resolv(?:e|es|ed))(\s*:?\s+)` +
			`((?:[\w.-]+/[\w.-]+)?#\d+|https?://github\.com/\S+)`)
)

// neutralize defuses what GitHub acts on in text an agent wrote: a mention would
// notify someone, and a closing keyword would close an issue on merge. A
// zero-width space breaks both and shows as nothing.
func neutralize(text string) string {
	text = mention.ReplaceAllString(text, "${1}@"+zeroWidth+"${2}")
	return closing.ReplaceAllString(text, "${1}${2}"+zeroWidth+"${3}")
}

// pullRequestText is the title and body of a job's pull request: the title is the
// first line of what the agent wrote, without a "PR title:" label or heading
// marks; the body is all of it, with a footer naming the task. The agent's text
// is the only thing in it that isn't the runner's own.
func pullRequestText(
	outcome string,
	job *ClaimedJob,
	serverURL string,
) (string, string) {
	title := ""
	for line := range strings.SplitSeq(outcome, "\n") {
		if titleLabel.MatchString(line) {
			title = line
			break
		}
		if title == "" && strings.TrimSpace(line) != "" {
			title = line
		}
	}
	title = titlePrefix.ReplaceAllString(title, "")
	title = strings.TrimSpace(strings.TrimLeft(title, "# \t"))
	title = strings.Trim(title, "\"'`* ")
	title = strings.Join(strings.Fields(title), " ")
	if utf8.RuneCountInString(title) > maxTitleRunes {
		title = string([]rune(title)[:maxTitleRunes])
	}
	if title == "" {
		title = "Factory job on " + job.Branch
	}
	body := neutralize(strings.TrimSpace(outcome))
	if utf8.RuneCountInString(body) > maxBodyRunes {
		body = string([]rune(body)[:maxBodyRunes]) + "…"
	}
	footer := "\n\n---\nOpened by the factory for task `" + job.TaskID + "` (job `" +
		job.ID + "`). Review it like any other change."
	if serverURL != "" {
		footer += " Board: " + strings.TrimRight(serverURL, "/") + "/factory"
	}
	return title, body + footer
}

// pushEnv puts the token where git reads config without it appearing in argv,
// which any process on the Mac can list: as an HTTP header for this push only. The
// Mac's own git config is switched off for the push: a developer's
// `url.<ssh>.insteadOf` would send it over SSH with a personal key, not the token.
func pushEnv(token string) []string {
	header := "AUTHORIZATION: basic " +
		base64.StdEncoding.EncodeToString([]byte("x-access-token:"+token))
	return []string{
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=http.extraHeader",
		"GIT_CONFIG_VALUE_0=" + header,
	}
}

// Push sends the reviewed branch to remote with the token. It never forces: a
// branch that already has other work on GitHub is left alone and the push fails.
func (r *Reviewed) Push(ctx context.Context, remote, token string) error {
	args := []string{
		"-c", "core.hooksPath=/dev/null", "-c", "credential.helper=",
		"push", "--no-verify", "--",
		remote, "refs/heads/" + r.branch + ":refs/heads/" + r.branch,
	}
	out, err := r.g.gitEnv(ctx, r.dir, pushEnv(token), args...)
	if err != nil {
		return fmt.Errorf("git push: %w: %s", err, scrub(string(bytes.TrimSpace(out)), token))
	}
	return nil
}

// scrub removes the token, and the header that carries it, from text that may be
// shown to a person.
func scrub(text, token string) string {
	if token == "" {
		return text
	}
	basic := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
	return strings.NewReplacer(token, redacted, basic, redacted).Replace(text)
}

// gitEnv is git with extra environment: the token for one push. A test's Run
// stands in for it, and RunWithEnv for a Run that wants to see the environment.
func (g *Git) gitEnv(
	ctx context.Context,
	dir string,
	env []string,
	args ...string,
) ([]byte, error) {
	if g.RunWithEnv != nil {
		return g.RunWithEnv(ctx, dir, env, args...)
	}
	if g.Run != nil {
		return g.Run(ctx, dir, args...)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	return out.Bytes(), err //nolint:wrapcheck // the caller names it
}

// pushToken asks the server for the job's push token, trying again when the
// failure may pass (the server reaching GitHub). A final answer is not retried.
func (r *JobRunner) pushToken(ctx context.Context, jobID string) (string, error) {
	var token string
	err := r.retry(ctx, func() error {
		var err error
		token, err = r.API.GetPushToken(ctx, r.Token, jobID)
		if PushUnavailable(err) {
			return &finalError{err}
		}
		return err
	})
	return token, err
}

// openPullRequest is GitHub's call, tried again a few times: the branch is
// already pushed by now, so a hiccup shouldn't waste it.
func (r *JobRunner) openPullRequest(
	ctx context.Context,
	token string,
	pr PullRequest,
) (string, error) {
	var url string
	err := r.retry(ctx, func() error {
		var err error
		url, err = r.GitHub.OpenPullRequest(ctx, token, pr)
		return err
	})
	return url, err
}

// finalError marks a failure that trying again can't change.
type finalError struct{ err error }

func (e *finalError) Error() string { return e.err.Error() }
func (e *finalError) Unwrap() error { return e.err }

func (r *JobRunner) retry(ctx context.Context, fn func() error) error {
	var err error
	for try := range publishTries {
		if err = fn(); err == nil {
			return nil
		}
		var final *finalError
		if errors.As(err, &final) || try == publishTries-1 {
			break
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(cmp.Or(r.PublishRetry, PublishRetry)):
		}
	}
	return err
}
