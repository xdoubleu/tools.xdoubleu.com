package services

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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
		ID: id, Number: 0, RepoID: uuid.Nil, Title: title, Body: body, Status: "",
		Priority: "", Labels: nil, CreatedBy: "", CreatedAt: time.Time{},
		UpdatedAt: time.Time{}, CommentCount: 0, PRURL: "", PRState: "", PRCI: "",
		PRReview: "", PickupApproved: false, PickupBlocked: "", MergeSHA: "", Deploy: "",
	}
}

func testRepo(instructions string) *models.Repo {
	return &models.Repo{
		ID: uuid.Nil, Owner: "", Name: "", DefaultBranch: "", DefaultHarness: "",
		DefaultModel: "", AutoPickup: false, SetupCommands: "",
		AgentInstructions: instructions,
		TimeLimitMinutes:  0,
		CreatedAt:         time.Time{}, UpdatedAt: time.Time{},
	}
}

func TestBranchForIsAgentNumberSlug(t *testing.T) {
	task := testTask(uuid.Nil, "Fix it", "")
	task.Number = 142
	assert.Equal(t, "agent/t142-fix-it", branchFor(task))
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

// The ends of each kept range survive: a, z, 0 and 9 are letters and digits, and
// the characters just outside them (` { / :) are separators.
func TestSlugKeepsTheEndsOfItsRanges(t *testing.T) {
	assert.Equal(t, "az09", slug("az09"))
	assert.Equal(t, "a-z-0-9", slug("a`z{0/9:"))
	assert.Equal(t, "b-y", slug("b`{y"))
}

func TestTruncateCutsOnARuneBoundary(t *testing.T) {
	assert.Equal(t, "short", truncate("short"))
	long := strings.Repeat("é", maxEventBytes) // two bytes each
	got := truncate(long)
	assert.LessOrEqual(t, len(got), maxEventBytes)
	assert.True(t, strings.HasPrefix(long, got))
	got = truncate("a" + strings.Repeat("é", maxEventBytes)) // cut lands mid-rune
	assert.True(t, utf8.ValidString(got))
}

func TestTheLimitsAreInclusive(t *testing.T) {
	exact := strings.Repeat("a", maxEventBytes)
	assert.Equal(t, exact, truncate(exact), "a line of exactly the limit is kept whole")
	assert.Len(t, truncate(exact+"a"), maxEventBytes)

	kind := strings.Repeat("k", maxKindLen)
	assert.Equal(t, kind, kindOf(`{"type":"`+kind+`"}`))
	assert.Equal(t, kind, kindOf(`{"type":"`+kind+`","cut`))
}

func TestKindOfIsTheLinesTypeOrRaw(t *testing.T) {
	assert.Equal(t, "assistant", kindOf(`{"type":"assistant"}`))
	assert.Equal(t, rawKind, kindOf(`{"type":""}`))
	assert.Equal(t, rawKind, kindOf(`{"other":1}`))
	assert.Equal(t, rawKind, kindOf(`not json`))
	// A cut line is no longer JSON, but its type is still at the front.
	assert.Equal(t, "user", kindOf(`{"type":"user","message":{"content":"cut o`))
	assert.Equal(t, rawKind, kindOf(`{"message":"cut`))
	assert.Equal(t, rawKind, kindOf(`{"type":"`+strings.Repeat("k", maxKindLen+1)+`"}`))
}

func TestLinkedTaskReadsTheNumberOrALegacyUUID(t *testing.T) {
	uuidRef := "11111111-2222-3333-4444-555555555555"
	for _, ref := range []string{"T-142", uuidRef} {
		got, ok := linkedTask("intro\nTask: Fix it (" + ref + ")\n")
		assert.True(t, ok, ref)
		assert.Equal(t, ref, got)
	}
	_, ok := linkedTask("Task: Fix it (142)")
	assert.False(t, ok)
}
