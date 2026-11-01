package services

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/factory/internal/models"
)

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Fix the Login Page!":      "fix-the-login-page",
		"  --leading & trailing--": "leading-trailing",
		"Ünïcode only: 日本語":        "n-code-only",
		"!!!":                      "task",
		"":                         "task",
		"UPPER_and_under":          "upper-and-under",
		strings.Repeat("a", 40):    strings.Repeat("a", maxSlugLen),
		strings.Repeat("ab ", 20):  "ab-ab-ab-ab-ab-ab-ab-ab-ab-ab",
	} {
		assert.Equal(t, want, slug(in), in)
	}
	// A cut that lands on a dash drops it: 29 letters, a space, then more.
	assert.Equal(t, strings.Repeat("a", 29), slug(strings.Repeat("a", 29)+" bbb"))
}

func testTask(id uuid.UUID, title, body string) *models.Task {
	return &models.Task{
		ID: id, RepoID: uuid.Nil, Title: title, Body: body, Status: "",
		Priority: "", Labels: nil, CreatedBy: "", CreatedAt: time.Time{},
		UpdatedAt: time.Time{}, CommentCount: 0,
	}
}

func testRepo(instructions string) *models.Repo {
	return &models.Repo{
		ID: uuid.Nil, Owner: "", Name: "", DefaultBranch: "", DefaultHarness: "",
		SetupCommands: "", AgentInstructions: instructions, TimeLimitMinutes: 0,
		CreatedAt: time.Time{}, UpdatedAt: time.Time{},
	}
}

func TestBranchForIsAgentIDSlug(t *testing.T) {
	id := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	got := branchFor(testTask(id, "Fix it", ""))
	assert.Equal(t, "agent/11111111-2222-3333-4444-555555555555-fix-it", got)
}

func TestPromptForJoinsInstructionsTaskAndRunText(t *testing.T) {
	repo := testRepo("  Be careful.  ")
	task := testTask(uuid.Nil, "T", "  body text  ")

	assert.Equal(t,
		"Be careful.\n\n# Task: T\n\nbody text\n\n# Instructions for this run\n\ngo",
		promptFor(repo, task, "go"))
	assert.Equal(t, "# Task: T\n\nbody text",
		promptFor(testRepo(""), task, ""), "no instructions, no run text")
}

func TestJobStatusFinished(t *testing.T) {
	for status, want := range map[models.JobStatus]bool{
		models.JobQueued: false, models.JobRunning: false,
		models.JobSucceeded: true, models.JobFailed: true,
		models.JobCancelled: true, models.JobLost: true,
	} {
		assert.Equal(t, want, status.Finished(), string(status))
	}
}
