package jobs_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/taste/internal/jobs"
	"tools.xdoubleu.com/internal/logging"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

type fakeUsers struct {
	users []sharedmodels.User
	err   error
}

func (f fakeUsers) GetAllUsers(context.Context) ([]sharedmodels.User, error) {
	return f.users, f.err
}

type fakeProfiles struct {
	configured bool
	dueErr     error
	failFor    string
	asked      []string
	generated  map[string]bool // user -> could read books
}

func (f *fakeProfiles) Configured() bool { return f.configured }

func (f *fakeProfiles) Due(_ context.Context, ids []string) ([]string, error) {
	f.asked = ids
	return ids, f.dueErr
}

func (f *fakeProfiles) Generate(
	_ context.Context, id string, include func(string) bool,
) error {
	f.generated[id] = include("books")
	if id == f.failFor {
		return errors.New("boom")
	}
	return nil
}

// profilesFake is a fake failing Generate for failFor and Due with dueErr.
func profilesFake(configured bool, failFor string, dueErr error) *fakeProfiles {
	return &fakeProfiles{
		configured: configured, dueErr: dueErr, failFor: failFor, asked: nil,
		generated: map[string]bool{},
	}
}

// fakePicks records whose picks were generated.
type fakePicks struct {
	users []string
	err   error
}

func (f *fakePicks) Generate(_ context.Context, id string, _ func(string) bool) error {
	f.users = append(f.users, id)
	return f.err
}

func user(id string, role sharedmodels.Role, access ...string) sharedmodels.User {
	//nolint:exhaustruct // only what the job reads
	return sharedmodels.User{ID: id, Role: role, AppAccess: access}
}

func TestGenerateProfilesJob(t *testing.T) {
	users := fakeUsers{users: []sharedmodels.User{
		user("c", sharedmodels.RoleUser, "taste", "books"),
		user("a", sharedmodels.RoleAdmin),
		user("b", sharedmodels.RoleUser, "taste"),
		user("x", sharedmodels.RoleUser, "books"),
	}, err: nil}
	profiles := profilesFake(true, "", nil)
	picks := &fakePicks{users: nil, err: nil}
	job := jobs.NewGenerateProfilesJob(users, profiles, picks, "taste")

	assert.Equal(t, jobs.GenerateProfilesID, job.ID())
	assert.Positive(t, job.RunEvery())
	require.NoError(t, job.Run(context.Background(), logging.NewNopLogger()))
	assert.Equal(t, []string{"a", "b", "c"}, profiles.asked)
	assert.Equal(t, map[string]bool{"a": true, "b": false, "c": true}, profiles.generated)
	assert.Equal(t, []string{"a", "b", "c"}, picks.users)
}

func TestGenerateProfilesJob_Failures(t *testing.T) {
	users := fakeUsers{users: []sharedmodels.User{
		user("a", sharedmodels.RoleAdmin), user("b", sharedmodels.RoleAdmin),
	}, err: nil}

	profiles := profilesFake(true, "a", nil)
	picks := &fakePicks{users: nil, err: nil}
	err := jobs.NewGenerateProfilesJob(users, profiles, picks, "taste").
		Run(context.Background(), logging.NewNopLogger())
	require.ErrorContains(t, err, "user a: boom")
	assert.Len(t, profiles.generated, 2, "one failure doesn't stop the others")
	assert.Equal(t, []string{"b"}, picks.users, "no picks without a fresh profile")

	failingPicks := &fakePicks{users: nil, err: errors.New("judge down")}
	err = jobs.NewGenerateProfilesJob(users, profilesFake(true, "", nil), failingPicks,
		"taste").Run(context.Background(), logging.NewNopLogger())
	require.ErrorContains(t, err, "user a: judge down")
	require.ErrorContains(t, err, "user b: judge down")

	profiles = profilesFake(true, "", errors.New("db"))
	err = jobs.NewGenerateProfilesJob(users, profiles, picks, "taste").
		Run(context.Background(), logging.NewNopLogger())
	require.ErrorContains(t, err, "db")

	err = jobs.NewGenerateProfilesJob(fakeUsers{users: nil, err: errors.New("auth")},
		profilesFake(true, "", nil), picks, "taste").
		Run(context.Background(), logging.NewNopLogger())
	require.ErrorContains(t, err, "auth")
}

func TestGenerateProfilesJob_IdleWithoutKey(t *testing.T) {
	profiles := profilesFake(false, "", nil)
	err := jobs.NewGenerateProfilesJob(fakeUsers{users: nil, err: errors.New("unused")},
		profiles, &fakePicks{users: nil, err: nil}, "taste").
		Run(context.Background(), logging.NewNopLogger())
	require.NoError(t, err)
	assert.Nil(t, profiles.asked)
}
