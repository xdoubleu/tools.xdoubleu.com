package services

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

// repoInput builds a RepoInput that passes validation, then lets mod edit it.
func repoInput(mod func(*RepoInput)) RepoInput {
	in := RepoInput{
		Owner:             "octo",
		Name:              "repo",
		DefaultBranch:     "main",
		TimeLimitMinutes:  60,
		AutoPickup:        false,
		SentryProject:     "",
		RedPRLabel:        "",
		RedPRBranchPrefix: "",
	}
	mod(&in)
	return in
}

func TestValidBranch(t *testing.T) {
	for _, ok := range []string{"main", "release/1.2", "feat_x-1", "a.b", "a.lockx"} {
		assert.True(t, validBranch(ok), ok)
	}
	for _, bad := range []string{
		"", "-x", "--upload-pack=x", "/a", "a/", "a..b", "a//b", "a b", "a~b", "a:b",
		".hidden", "a/.b", "a.lock", "feature/x.lock", "foo.", "@",
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
		r.TimeLimitMinutes = 0
	}))
	require.NoError(t, err)
	assert.Equal(t, "octo", got.Owner)
	assert.Equal(t, "repo", got.Name)
	assert.Equal(t, "main", got.DefaultBranch)
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

	bad(func(r *RepoInput) { r.DefaultBranch = "-x" })
}

func TestUpdateRequiresEverySetting(t *testing.T) {
	svc := &RepoService{repo: nil}
	for name, mod := range map[string]func(*RepoInput){
		"no branch":     func(r *RepoInput) { r.DefaultBranch = "  " },
		"no time limit": func(r *RepoInput) { r.TimeLimitMinutes = 0 },
	} {
		_, err := svc.Update(
			context.Background(),
			models.Actor{Kind: models.ActorHuman, ID: "x", Browser: true, Admin: true, Via: ""},
			uuid.New(), repoInput(mod))
		assert.Equal(t, http.StatusBadRequest, status(t, err), name)
	}
}

func TestNormaliseRepoSignalSettings(t *testing.T) {
	got, err := normaliseRepo(repoInput(func(r *RepoInput) {
		r.SentryProject = " check-in "
		r.RedPRLabel = " deps "
		r.RedPRBranchPrefix = " agent/ "
	}))
	require.NoError(t, err)
	assert.Equal(t, "check-in", got.SentryProject)
	assert.Equal(t, "deps", got.RedPRLabel)
	assert.Equal(t, "agent/", got.RedPRBranchPrefix)

	for name, mod := range map[string]func(*RepoInput){
		"sentry slug upper": func(r *RepoInput) { r.SentryProject = "Check-In" },
		"sentry slug space": func(r *RepoInput) { r.SentryProject = "a b" },
		"sentry slug dash":  func(r *RepoInput) { r.SentryProject = "-a" },
		"label too long": func(r *RepoInput) {
			r.RedPRLabel = strings.Repeat("a", maxRepoLabelLen+1)
		},
		"prefix has space": func(r *RepoInput) { r.RedPRBranchPrefix = "a b/" },
		"prefix has tilde": func(r *RepoInput) { r.RedPRBranchPrefix = "a~/" },
		"prefix too long": func(r *RepoInput) {
			r.RedPRBranchPrefix = strings.Repeat("a", maxBranchLen+1)
		},
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

func TestRepoSettingsNeedABrowserSession(t *testing.T) {
	svc := &RepoService{repo: nil}
	bearer := models.Actor{Kind: models.ActorHuman, ID: "x", Browser: false, Admin: true, Via: ""}

	_, err := svc.Create(context.Background(), bearer, repoInput(func(*RepoInput) {}))
	assert.Equal(t, http.StatusForbidden, status(t, err))
	_, err = svc.Update(context.Background(), bearer, uuid.New(), repoInput(func(*RepoInput) {}))
	assert.Equal(t, http.StatusForbidden, status(t, err))
}
