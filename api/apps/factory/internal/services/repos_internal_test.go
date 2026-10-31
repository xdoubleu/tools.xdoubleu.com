package services

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/factory/internal/models"
)

// repoInput builds a RepoInput that passes validation, then lets mod edit it.
func repoInput(mod func(*RepoInput)) RepoInput {
	in := RepoInput{
		Owner:             "octo",
		Name:              "repo",
		DefaultBranch:     "main",
		DefaultHarness:    models.HarnessClaudeCode,
		SetupCommands:     "",
		AgentInstructions: "",
		TimeLimitMinutes:  60,
	}
	mod(&in)
	return in
}

func TestValidBranch(t *testing.T) {
	for _, ok := range []string{"main", "release/1.2", "feat_x-1", "a.b"} {
		assert.True(t, validBranch(ok), ok)
	}
	for _, bad := range []string{
		"", "-x", "--upload-pack=x", "/a", "a/", "a..b", "a//b", "a b", "a~b", "a:b",
		strings.Repeat("a", maxBranchLen+1),
	} {
		assert.False(t, validBranch(bad), bad)
	}
	assert.True(t, validBranch(strings.Repeat("a", maxBranchLen)))
}

func TestNormaliseRepoOwnerAndName(t *testing.T) {
	_, err := normaliseRepo(repoInput(func(*RepoInput) {}))
	require.NoError(t, err)

	for name, mod := range map[string]func(*RepoInput){
		"empty owner":    func(r *RepoInput) { r.Owner = "" },
		"owner slash":    func(r *RepoInput) { r.Owner = "a/b" },
		"owner dash":     func(r *RepoInput) { r.Owner = "-a" },
		"owner too long": func(r *RepoInput) { r.Owner = strings.Repeat("a", 40) },
		"empty name":     func(r *RepoInput) { r.Name = "" },
		"dot name":       func(r *RepoInput) { r.Name = "." },
		"dotdot name":    func(r *RepoInput) { r.Name = ".." },
		"name space":     func(r *RepoInput) { r.Name = "a b" },
		"name too long":  func(r *RepoInput) { r.Name = strings.Repeat("a", 101) },
	} {
		_, err = normaliseRepo(repoInput(mod))
		assert.Equal(t, http.StatusBadRequest, status(t, err), name)
	}

	_, err = normaliseRepo(repoInput(func(r *RepoInput) {
		r.Owner = strings.Repeat("a", 39)
		r.Name = strings.Repeat("b", 100)
	}))
	assert.NoError(t, err, "limits are inclusive")
}

func TestNormaliseRepoTrimsAndDefaults(t *testing.T) {
	got, err := normaliseRepo(repoInput(func(r *RepoInput) {
		r.Owner = " octo "
		r.Name = " repo "
		r.DefaultBranch = ""
		r.DefaultHarness = ""
		r.TimeLimitMinutes = 0
	}))
	require.NoError(t, err)
	assert.Equal(t, "octo", got.Owner)
	assert.Equal(t, "repo", got.Name)
	assert.Equal(t, "main", got.DefaultBranch)
	assert.Equal(t, models.HarnessClaudeCode, got.DefaultHarness)
	assert.Equal(t, defaultTimeLimit, got.TimeLimitMinutes)

	got, err = normaliseRepo(repoInput(func(r *RepoInput) { r.DefaultBranch = " dev " }))
	require.NoError(t, err)
	assert.Equal(t, "dev", got.DefaultBranch)
}

func TestNormaliseRepoBounds(t *testing.T) {
	ok := func(mod func(*RepoInput)) {
		t.Helper()
		_, err := normaliseRepo(repoInput(mod))
		assert.NoError(t, err)
	}
	bad := func(mod func(*RepoInput)) {
		t.Helper()
		_, err := normaliseRepo(repoInput(mod))
		assert.Equal(t, http.StatusBadRequest, status(t, err))
	}

	ok(func(r *RepoInput) { r.TimeLimitMinutes = minTimeLimit })
	ok(func(r *RepoInput) { r.TimeLimitMinutes = maxTimeLimit })
	bad(func(r *RepoInput) { r.TimeLimitMinutes = -1 })
	bad(func(r *RepoInput) { r.TimeLimitMinutes = maxTimeLimit + 1 })

	ok(func(r *RepoInput) { r.SetupCommands = strings.Repeat("a", maxSetupLen) })
	bad(func(r *RepoInput) { r.SetupCommands = strings.Repeat("a", maxSetupLen+1) })
	ok(
		func(r *RepoInput) { r.AgentInstructions = strings.Repeat("a", maxInstructionsLen) },
	)
	bad(func(r *RepoInput) {
		r.AgentInstructions = strings.Repeat("a", maxInstructionsLen+1)
	})

	ok(func(r *RepoInput) { r.DefaultHarness = models.HarnessOpenCode })
	bad(func(r *RepoInput) { r.DefaultHarness = models.HarnessInvalid })
	bad(func(r *RepoInput) { r.DefaultBranch = "-x" })
}
