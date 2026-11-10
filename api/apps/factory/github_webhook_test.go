package factory_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/factory"
	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/internal/testhelper"
)

const webhookSecret = "hook-secret"

func sign(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func pullEvent(t *testing.T, action string, merged bool, body string) string {
	t.Helper()
	payload := map[string]any{
		"action": action,
		"pull_request": map[string]any{
			"html_url": pullURL(7), "body": body, "merged": merged,
			"merge_commit_sha": "hooksha",
		},
	}
	out, err := json.Marshal(payload)
	require.NoError(t, err)
	return string(out)
}

// deliver posts a webhook delivery to the factory and returns the status.
func deliver(
	t *testing.T, env jobEnv, event, body, signature string,
) int {
	t.Helper()
	ts := httptest.NewServer(testhelper.BuildMux(env.app))
	t.Cleanup(ts.Close)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		ts.URL+"/factory/github-webhook", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("X-GitHub-Event", event)
	req.Header.Set("X-Hub-Signature-256", signature)
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

func webhookEnv(t *testing.T) jobEnv {
	t.Helper()
	secret := factory.WithGitHubWebhookSecret(webhookSecret)
	return newJobEnvWith(t, true, "browser", secret)
}

func TestAMergeWebhookMovesTheLinkedTaskToTesting(t *testing.T) {
	env := webhookEnv(t)
	task := env.newTask(t, "hooked")
	env.putInReview(t, task.Id)
	body := pullEvent(t, "closed", true, "Task: hooked ("+task.Id+")\n\nbody")

	status := deliver(t, env, "pull_request", body, sign(webhookSecret, body))

	assert.Equal(t, http.StatusNoContent, status)
	got := env.task(t, task.Id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_TESTING, got.Task.Status)
	require.Len(t, got.Comments, 1)
	assert.Contains(t, got.Comments[0].Body, "was merged: "+pullURL(7))
}

func TestAWebhookWithABadSignatureChangesNothing(t *testing.T) {
	env := webhookEnv(t)
	task := env.newTask(t, "forged")
	env.putInReview(t, task.Id)
	body := pullEvent(t, "closed", true, "Task: forged ("+task.Id+")")

	for _, signature := range []string{
		"", "sha256=zz", sign("other-secret", body),
		strings.TrimPrefix(sign(webhookSecret, body), "sha256="),
	} {
		assert.Equal(t, http.StatusUnauthorized,
			deliver(t, env, "pull_request", body, signature))
	}
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_REVIEW,
		env.task(t, task.Id).Task.Status)
}

func TestOtherWebhookEventsAreAcknowledgedAndIgnored(t *testing.T) {
	env := webhookEnv(t)
	task := env.newTask(t, "ignored")
	env.putInReview(t, task.Id)
	link := "Task: ignored (" + task.Id + ")"

	for _, tc := range []struct{ event, body string }{
		{"ping", `{"zen":"keep it logically awesome"}`},
		{"pull_request", pullEvent(t, "closed", false, link)},
		{"pull_request", pullEvent(t, "opened", false, link)},
		{"pull_request", `{"action":"closed","pull_request":{"merged":true,"body":null}}`},
	} {
		assert.Equal(t, http.StatusNoContent,
			deliver(t, env, tc.event, tc.body, sign(webhookSecret, tc.body)))
	}
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_REVIEW,
		env.task(t, task.Id).Task.Status)
}

func TestAMalformedSignedWebhookIsABadRequest(t *testing.T) {
	env := webhookEnv(t)
	body := "{not json"

	assert.Equal(t, http.StatusBadRequest,
		deliver(t, env, "pull_request", body, sign(webhookSecret, body)))
}

func TestTheWebhookIsOffWithoutASecret(t *testing.T) {
	env := newJobEnvWith(t, true, "browser")
	body := pullEvent(t, "closed", true, "no link")

	assert.Equal(t, http.StatusServiceUnavailable,
		deliver(t, env, "pull_request", body, sign("", body)))
}

func TestAMergeWebhookStillFollowsAPullRequestLinkedByUUID(t *testing.T) {
	env := webhookEnv(t)
	task := env.newTask(t, "legacy")
	env.putInReview(t, task.Id)
	key := taskKey(t, task.Id)
	body := pullEvent(t, "closed", true, "Task: legacy ("+key.String()+")\n\nbody")

	status := deliver(t, env, "pull_request", body, sign(webhookSecret, body))

	assert.Equal(t, http.StatusNoContent, status)
	got := env.task(t, task.Id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_TESTING, got.Task.Status)
}
