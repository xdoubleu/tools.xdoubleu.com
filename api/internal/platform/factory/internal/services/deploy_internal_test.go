package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

type stubGitHub struct{ GitHub }

func deployTask(sha string, status models.Status) models.Task {
	var t models.Task
	t.MergeSHA, t.Status, t.UpdatedAt = sha, status, time.Unix(1000, 0)
	return t
}

func TestFillDeployGivesAReasonForEachUnknown(t *testing.T) {
	cases := []struct {
		name    string
		github  GitHub
		release string
		task    models.Task
		want    models.DeployState
		reason  string
	}{
		{"no merge commit", nil, "abc", deployTask("", models.StatusTesting),
			models.DeployUnknown, ReasonNoMergeCommit},
		{"no github app", nil, "abc", deployTask("sha", models.StatusTesting),
			models.DeployUnknown, ReasonNoGitHub},
		{
			"dev release",
			stubGitHub{GitHub: nil},
			devRelease,
			deployTask("sha", models.StatusDone),
			models.DeployUnknown,
			ReasonNoRelease,
		},
		{"empty release", stubGitHub{GitHub: nil}, "", deployTask("sha", models.StatusDone),
			models.DeployUnknown, ReasonNoRelease},
		{"done without a commit says nothing", nil, "abc",
			deployTask("", models.StatusDone), models.DeployNone, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := &JobService{ //nolint:exhaustruct // only what FillDeploy reads
				github: c.github, release: c.release, deploys: newDeployCache(),
				now: func() time.Time { return time.Unix(2000, 0) },
			}
			tasks := []models.Task{c.task}
			svc.FillDeploy(context.Background(), tasks)
			assert.Equal(t, c.want, tasks[0].Deploy)
			assert.Equal(t, c.reason, tasks[0].DeployReason)
		})
	}
}
