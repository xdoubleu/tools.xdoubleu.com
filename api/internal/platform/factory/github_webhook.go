package factory

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"tools.xdoubleu.com/internal/platform/factory/internal/services"
)

// webhookMaxBody caps a delivery; the fields read here sit in a pull request
// body.
const webhookMaxBody = 1 << 20

// WithGitHubWebhookSecret turns on POST /factory/github-webhook, which the
// factory GitHub App calls when a pull request changes. Without a secret the
// endpoint answers 503 and only the poll moves merged tasks.
func WithGitHubWebhookSecret(secret string) Option {
	return func(a *Factory) { a.webhookSecret = secret }
}

// validSignature checks GitHub's `sha256=<hex>` HMAC of the body.
func validSignature(secret string, body []byte, header string) bool {
	sig, ok := strings.CutPrefix(header, "sha256=")
	if !ok {
		return false
	}
	got, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(got, mac.Sum(nil))
}

type pullRequestEvent struct {
	Action      string `json:"action"`
	PullRequest struct {
		URL    string  `json:"html_url"`
		Title  string  `json:"title"`
		Body   *string `json:"body"`
		State  string  `json:"state"`
		Merged bool    `json:"merged"`
		SHA    string  `json:"merge_commit_sha"`
		Head   struct {
			Ref string `json:"ref"`
		} `json:"head"`
	} `json:"pull_request"`
}

// githubWebhook links the tasks a pull request names and moves them: In review
// while it is open, Testing once it merges; a close without a merge only leaves a
// comment. The signature is the only authentication; other events are
// acknowledged and ignored.
func (a *Factory) githubWebhook(w http.ResponseWriter, r *http.Request) {
	if a.webhookSecret == "" {
		http.Error(w, "not configured", http.StatusServiceUnavailable)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, webhookMaxBody))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !validSignature(a.webhookSecret, body, r.Header.Get("X-Hub-Signature-256")) {
		a.Logger.WarnContext(r.Context(), "github webhook: bad signature")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Header.Get("X-GitHub-Event") != "pull_request" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var event pullRequestEvent
	if err = json.Unmarshal(body, &event); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	pr := event.PullRequest
	ref := services.PullRequestRef{
		URL: pr.URL, Title: pr.Title, Branch: pr.Head.Ref, Body: "", MergeSHA: pr.SHA,
	}
	if pr.Body != nil {
		ref.Body = *pr.Body
	}
	switch {
	case event.Action == "closed" && pr.Merged:
		_, err = a.services.Jobs.TestingAfterMerge(r.Context(), ref)
	case event.Action == "closed":
		err = a.services.Jobs.PullRequestClosed(r.Context(), ref)
	case pr.State == "open" && opensOrEdits(event.Action):
		err = a.services.Jobs.PullRequestOpened(r.Context(), ref)
	default:
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		// GitHub doesn't retry, so the poll is what recovers this.
		a.Logger.ErrorContext(r.Context(), "github webhook: moving the task failed",
			"error", err, "pullRequest", pr.URL)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// opensOrEdits are the events after which a pull request is open and may name
// tasks it did not before.
func opensOrEdits(action string) bool {
	switch action {
	case "opened", "reopened", "edited", "ready_for_review":
		return true
	}
	return false
}
