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
	due        []string        // when set, the only users due
}

func (f *fakeProfiles) Configured() bool { return f.configured }

func (f *fakeProfiles) Due(_ context.Context, ids []string) ([]string, error) {
	f.asked = ids
	if f.due != nil {
		return f.due, f.dueErr
	}
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
		generated: map[string]bool{}, due: nil,
	}
}

// fakePicks records whose picks were generated.
type fakePicks struct {
	users  []string
	err    error
	due    []string // users whose picks are due
	dueErr error
	asked  []string
}

func (f *fakePicks) Due(_ context.Context, ids []string) ([]string, error) {
	f.asked = ids
	return f.due, f.dueErr
}

func (f *fakePicks) Generate(_ context.Context, id string, _ func(string) bool) error {
	f.users = append(f.users, id)
	return f.err
}

func newPicks(err error) *fakePicks {
	return &fakePicks{users: nil, err: err, due: nil, dueErr: nil, asked: nil}
}

// fakeNotify records who was told about new picks, with their address.
type fakeNotify struct {
	told []string
}

func (f *fakeNotify) NotifyNewPicks(_ context.Context, id, email string) {
	f.told = append(f.told, id+"="+email)
}

func noNotify() *fakeNotify { return &fakeNotify{told: nil} }

func user(id string, role sharedmodels.Role, access ...string) sharedmodels.User {
	//nolint:exhaustruct // only what the job reads
	return sharedmodels.User{
		ID: id, Email: id + "@example.com", Role: role, AppAccess: access,
	}
}

func TestGenerateProfilesJob(t *testing.T) {
	users := fakeUsers{users: []sharedmodels.User{
		user("c", sharedmodels.RoleUser, "taste", "books"),
		user("a", sharedmodels.RoleAdmin),
		user("b", sharedmodels.RoleUser, "taste"),
		user("x", sharedmodels.RoleUser, "books"),
	}, err: nil}
	profiles := profilesFake(true, "", nil)
	picks := newPicks(nil)
	notify := noNotify()
	job := jobs.NewGenerateProfilesJob(users, profiles, picks, notify, "taste")

	assert.Equal(t, jobs.GenerateProfilesID, job.ID())
	assert.Positive(t, job.RunEvery())
	require.NoError(t, job.Run(context.Background(), logging.NewNopLogger()))
	assert.Equal(t, []string{"a", "b", "c"}, profiles.asked)
	assert.Equal(t, map[string]bool{"a": true, "b": false, "c": true}, profiles.generated)
	assert.Equal(t, []string{"a", "b", "c"}, picks.users)
	assert.Equal(t,
		[]string{"a=a@example.com", "b=b@example.com", "c=c@example.com"}, notify.told)
}

func TestGenerateProfilesJob_Failures(t *testing.T) {
	users := fakeUsers{users: []sharedmodels.User{
		user("a", sharedmodels.RoleAdmin), user("b", sharedmodels.RoleAdmin),
	}, err: nil}

	profiles := profilesFake(true, "a", nil)
	picks := newPicks(nil)
	notify := noNotify()
	err := jobs.NewGenerateProfilesJob(users, profiles, picks, notify, "taste").
		Run(context.Background(), logging.NewNopLogger())
	require.ErrorContains(t, err, "user a: boom")
	assert.Equal(t, []string{"b=b@example.com"}, notify.told,
		"only after picks were written")
	assert.Len(t, profiles.generated, 2, "one failure doesn't stop the others")
	assert.Equal(t, []string{"b"}, picks.users, "no picks without a fresh profile")

	failingPicks := newPicks(errors.New("picks down"))
	notify = noNotify()
	err = jobs.NewGenerateProfilesJob(users, profilesFake(true, "", nil), failingPicks,
		notify, "taste").Run(context.Background(), logging.NewNopLogger())
	require.ErrorContains(t, err, "user a: picks down")
	require.ErrorContains(t, err, "user b: picks down")
	assert.Empty(t, notify.told, "no email for a failed run")

	profiles = profilesFake(true, "", errors.New("db"))
	err = jobs.NewGenerateProfilesJob(users, profiles, picks, noNotify(), "taste").
		Run(context.Background(), logging.NewNopLogger())
	require.ErrorContains(t, err, "db")

	err = jobs.NewGenerateProfilesJob(fakeUsers{users: nil, err: errors.New("auth")},
		profilesFake(true, "", nil), picks, noNotify(), "taste").
		Run(context.Background(), logging.NewNopLogger())
	require.ErrorContains(t, err, "auth")
}

func TestGenerateProfilesJob_RetriesPicks(t *testing.T) {
	users := fakeUsers{users: []sharedmodels.User{
		user("a", sharedmodels.RoleAdmin), user("b", sharedmodels.RoleUser, "taste"),
	}, err: nil}
	profiles := &fakeProfiles{
		configured: true, dueErr: nil, failFor: "", asked: nil,
		generated: map[string]bool{}, due: []string{"a"},
	}
	picks := newPicks(nil)
	picks.due = []string{"b"}
	notify := noNotify()

	err := jobs.NewGenerateProfilesJob(users, profiles, picks, notify, "taste").
		Run(context.Background(), logging.NewNopLogger())
	require.NoError(t, err)
	assert.Equal(t, []string{"b"}, picks.asked, "not asked again about a user just done")
	assert.Equal(t, []string{"a", "b"}, picks.users)
	assert.Equal(t, map[string]bool{"a": true}, profiles.generated,
		"the retry keeps the stored profile")
	assert.Equal(t, []string{"a=a@example.com", "b=b@example.com"}, notify.told)

	picks = newPicks(errors.New("picks down"))
	picks.due = []string{"b"}
	notify = noNotify()
	profiles.due = nil
	err = jobs.NewGenerateProfilesJob(users, profiles, picks, notify, "taste").
		Run(context.Background(), logging.NewNopLogger())
	require.ErrorContains(t, err, "user b: picks down")
	assert.Empty(t, notify.told)

	picks = newPicks(nil)
	picks.dueErr = errors.New("db")
	err = jobs.NewGenerateProfilesJob(users, profiles, picks, noNotify(), "taste").
		Run(context.Background(), logging.NewNopLogger())
	require.ErrorContains(t, err, "db")
}

func TestGenerateProfilesJob_IdleWithoutKey(t *testing.T) {
	profiles := profilesFake(false, "", nil)
	err := jobs.NewGenerateProfilesJob(fakeUsers{users: nil, err: errors.New("unused")},
		profiles, newPicks(nil), noNotify(), "taste").
		Run(context.Background(), logging.NewNopLogger())
	require.NoError(t, err)
	assert.Nil(t, profiles.asked)
}

func TestGenerateProfilesJob_SharedSubjects(t *testing.T) {
	users := fakeUsers{users: []sharedmodels.User{
		user("a", sharedmodels.RoleAdmin),
	}, err: nil}
	profiles := profilesFake(true, "", nil)
	picks := newPicks(nil)
	notify := noNotify()
	job := jobs.NewGenerateProfilesJob(users, profiles, picks, notify, "taste").
		WithShared(func(context.Context) ([]string, error) {
			return []string{"family:f1"}, nil
		})

	require.NoError(t, job.Run(context.Background(), logging.NewNopLogger()))
	assert.Equal(t, []string{"a", "family:f1"}, profiles.asked)
	assert.Equal(t, map[string]bool{"a": true, "family:f1": false}, profiles.generated,
		"a shared profile reads no other app than movies")
	assert.Equal(t, []string{"a", "family:f1"}, picks.users)
	assert.Equal(t, []string{"a=a@example.com"}, notify.told, "nobody is emailed for it")

	failing := jobs.NewGenerateProfilesJob(users, profiles, picks, notify, "taste").
		WithShared(func(context.Context) ([]string, error) {
			return nil, errors.New("movies down")
		})
	require.ErrorContains(t, failing.Run(context.Background(), logging.NewNopLogger()),
		"movies down")
}
