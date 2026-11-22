package services

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

func ptr[T any](v T) *T { return &v }

func boardTask(status models.Status) *models.Task {
	//nolint:exhaustruct // only these fields matter here
	return &models.Task{
		Title: "Old title", Body: "old body", Priority: models.PriorityP1,
		Labels: []string{"docs", "Platform"}, Status: status,
	}
}

// patch is a TaskPatch of only the fields given.
func patch(mod func(*TaskPatch)) TaskPatch {
	p := TaskPatch{
		Title: nil, Body: nil, Priority: nil, Labels: nil, AddLabels: nil,
		RemoveLabels: nil,
	}
	mod(&p)
	return p
}

func actorOf(kind models.ActorKind, browser bool) models.Actor {
	return models.Actor{Kind: kind, ID: "u", Browser: browser, Admin: browser, Via: ""}
}

func TestMergeLabelsRemovesThenAddsIgnoringCase(t *testing.T) {
	got := mergeLabels(
		[]string{"docs", "Platform"}, []string{"epic:x", "DOCS"}, []string{"platform"})
	assert.Equal(t, []string{"docs", "epic:x"}, got)
	assert.Equal(t, []string{}, mergeLabels(nil, nil, nil), "never nil")
}

func TestApplyPatchKeepsWhatItLeavesOut(t *testing.T) {
	agent := actorOf(models.ActorAgent, false)
	backlog := boardTask(models.StatusBacklog)
	in, err := applyPatch(agent, backlog, patch(func(p *TaskPatch) {
		p.Title = ptr("New title")
		p.AddLabels = []string{"epic:x"}
		p.RemoveLabels = []string{"platform"}
	}))
	require.NoError(t, err)
	assert.Equal(t, "New title", in.Title)
	assert.Equal(t, "old body", in.Body)
	assert.Equal(t, models.PriorityP1, in.Priority)
	assert.Equal(t, []string{"docs", "epic:x"}, in.Labels)

	in, err = applyPatch(agent, backlog, patch(func(p *TaskPatch) {
		p.Body = ptr("new body")
		p.Priority = ptr(models.PriorityP0)
		p.Labels = ptr([]string{"only"})
	}))
	require.NoError(t, err)
	assert.Equal(t, "Old title", in.Title)
	assert.Equal(t, "new body", in.Body)
	assert.Equal(t, models.PriorityP0, in.Priority)
	assert.Equal(t, []string{"only"}, in.Labels)
}

func TestApplyPatchReadyTextIsForPeopleInTheBrowser(t *testing.T) {
	ready := boardTask(models.StatusReady)
	agent := actorOf(models.ActorAgent, false)
	browser := actorOf(models.ActorHuman, true)
	person := actorOf(models.ActorHuman, false)

	for _, edit := range []TaskPatch{
		patch(func(p *TaskPatch) { p.Body = ptr("sneaky rewrite") }),
		patch(func(p *TaskPatch) { p.Title = ptr("sneaky title") }),
	} {
		_, err := applyPatch(agent, ready, edit)
		assert.Equal(t, http.StatusForbidden, status(t, err))
		_, err = applyPatch(browser, ready, edit)
		assert.NoError(t, err)
		_, err = applyPatch(person, ready, edit)
		assert.NoError(t, err)
	}

	// Labels and priority stay open to an agent, and so does restating the text.
	in, err := applyPatch(agent, ready, patch(func(p *TaskPatch) {
		p.Title = ptr("Old title")
		p.Priority = ptr(models.PriorityP0)
		p.AddLabels = []string{"extra"}
	}))
	require.NoError(t, err)
	assert.Equal(t, []string{"docs", "Platform", "extra"}, in.Labels)
}

func TestCheckBulkRefusesBadInput(t *testing.T) {
	for name, tc := range map[string]struct {
		refs   []string
		status models.Status
	}{
		"no ids":    {[]string{}, models.StatusDone},
		"too many":  {make([]string, MaxBulkMove+1), models.StatusDone},
		"bad state": {[]string{"T-1"}, "later"},
	} {
		_, err := checkBulk(tc.refs, tc.status)
		require.Error(t, err, name)
		assert.Equal(t, http.StatusBadRequest, status(t, err), name)
	}
}

func TestCheckBulkSortsAndDedupes(t *testing.T) {
	got, err := checkBulk([]string{"T-2", "T-1", "T-2"}, models.StatusDone)
	require.NoError(t, err)
	assert.Equal(t, []string{"T-1", "T-2"}, got)

	refs := make([]string, MaxBulkMove)
	for i := range refs {
		refs[i] = uuid.NewString()
	}
	_, err = checkBulk(refs, models.StatusDone)
	assert.NoError(t, err, "exactly the limit is accepted")
}

func TestScopeConfinesOnlyAJobsOwnTask(t *testing.T) {
	assert.False(t, scope{scoped: false, task: uuid.New()}.confined())
	assert.False(t, scope{scoped: true, task: uuid.Nil}.confined(), "routine job")
	assert.True(t, scope{scoped: true, task: uuid.New()}.confined())
}

func TestListTasksRefusesAnUnknownStatus(t *testing.T) {
	b := &BoardService{tasks: nil, jobs: nil}
	_, err := b.ListTasks(t.Context(), actorOf(models.ActorHuman, true),
		models.TaskFilter{Query: "", Status: "archived", RepoID: nil, Label: ""})
	require.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, status(t, err))
	assert.True(t, strings.Contains(err.Error(), StatusNames))
}
