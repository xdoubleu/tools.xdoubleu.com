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
	job := jobs.NewGenerateProfilesJob(users, profiles, "taste")

	assert.Equal(t, jobs.GenerateProfilesID, job.ID())
	assert.Positive(t, job.RunEvery())
	require.NoError(t, job.Run(context.Background(), logging.NewNopLogger()))
	assert.Equal(t, []string{"a", "b", "c"}, profiles.asked)
	assert.Equal(t, map[string]bool{"a": true, "b": false, "c": true}, profiles.generated)
}

func TestGenerateProfilesJob_Failures(t *testing.T) {
	users := fakeUsers{users: []sharedmodels.User{
		user("a", sharedmodels.RoleAdmin), user("b", sharedmodels.RoleAdmin),
	}, err: nil}

	profiles := profilesFake(true, "a", nil)
	err := jobs.NewGenerateProfilesJob(users, profiles, "taste").
		Run(context.Background(), logging.NewNopLogger())
	require.ErrorContains(t, err, "user a: boom")
	assert.Len(t, profiles.generated, 2, "one failure doesn't stop the others")

	profiles = profilesFake(true, "", errors.New("db"))
	err = jobs.NewGenerateProfilesJob(users, profiles, "taste").
		Run(context.Background(), logging.NewNopLogger())
	require.ErrorContains(t, err, "db")

	err = jobs.NewGenerateProfilesJob(fakeUsers{users: nil, err: errors.New("auth")},
		profilesFake(true, "", nil), "taste").
		Run(context.Background(), logging.NewNopLogger())
	require.ErrorContains(t, err, "auth")
}

func TestGenerateProfilesJob_IdleWithoutKey(t *testing.T) {
	profiles := profilesFake(false, "", nil)
	err := jobs.NewGenerateProfilesJob(fakeUsers{users: nil, err: errors.New("unused")},
		profiles, "taste").Run(context.Background(), logging.NewNopLogger())
	require.NoError(t, err)
	assert.Nil(t, profiles.asked)
}
