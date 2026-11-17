package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

type filesGitHub struct {
	GitHub
	files []string
}

func (g filesGitHub) PullRequestFiles(
	context.Context, string, string, int,
) ([]string, error) {
	return g.files, nil
}

func TestSelfCheckedRuleSendsADRsToAPerson(t *testing.T) {
	assert.Equal(t, "", selfCheckedRule("docs/adr-0054-agent-job-sandbox-spike.md"))
	assert.Equal(t, "docs", selfCheckedRule("docs/convention-task-lifecycle.md"))
	assert.Equal(t, "docs", selfCheckedRule("README.md"))
	assert.Equal(t, "", selfCheckedBy([]string{"README.md", "docs/adr-0001-x.md"}))
}

func TestAfterMergeStatusADRGoesToTesting(t *testing.T) {
	const pr = "https://github.com/xdoubleu/tools.xdoubleu.com/pull/7"
	tests := []struct {
		name   string
		labels []string
		files  []string
		want   models.Status
		note   bool
	}{
		{"adr only", nil, []string{"docs/adr-0054-x.md"}, models.StatusTesting, true},
		{"plain docs", nil, []string{"docs/convention-x.md"}, models.StatusDone, false},
		{"sandbox label with adr", []string{"factory-sandbox"},
			[]string{"docs/adr-0054-x.md", "factory-runner/Containerfile"},
			models.StatusTesting, true},
		{"sandbox label without adr", []string{"factory-sandbox"},
			[]string{"web/a.tsx"}, models.StatusDone, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := new(JobService)
			s.github = filesGitHub{GitHub: nil, files: tt.files}
			task := new(models.Task)
			task.Labels = tt.labels
			got, note := s.afterMergeStatus(t.Context(), task, pr)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.note, note != "")
		})
	}
}
