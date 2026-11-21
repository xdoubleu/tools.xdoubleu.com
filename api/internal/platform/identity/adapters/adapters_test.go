//nolint:exhaustruct // fakes and fixtures set only the fields a test reads
package adapters_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/auth"
	"tools.xdoubleu.com/internal/platform/identity/adapters"
	"tools.xdoubleu.com/internal/platform/identity/agentaccounts"
)

type factorChecker struct{ err error }

func (f factorChecker) VerifyCurrentFactor(context.Context, string, string) error {
	return f.err
}

func TestStepUpTranslatesAuthErrorsAndKeepsTheirMessage(t *testing.T) {
	ctx := context.Background()

	err := adapters.StepUp{Auth: factorChecker{err: auth.ErrTooManyAttempts}}.
		VerifyCurrentFactor(ctx, "t", "1")
	require.ErrorIs(t, err, agentaccounts.ErrTooManyAttempts)
	require.ErrorIs(t, err, auth.ErrTooManyAttempts)
	assert.Equal(t, auth.ErrTooManyAttempts.Error(), err.Error())
	assert.NotErrorIs(t, err, agentaccounts.ErrWrongCredential)

	err = adapters.StepUp{Auth: factorChecker{err: auth.ErrWrongCredential}}.
		VerifyCurrentFactor(ctx, "t", "1")
	assert.ErrorIs(t, err, agentaccounts.ErrWrongCredential)
	assert.ErrorIs(t, err, auth.ErrWrongCredential)

	other := errors.New("db down")
	err = adapters.StepUp{
		Auth: factorChecker{err: other},
	}.VerifyCurrentFactor(
		ctx,
		"t",
		"1",
	)
	assert.Equal(t, other, err)

	assert.NoError(
		t,
		adapters.StepUp{Auth: factorChecker{}}.VerifyCurrentFactor(ctx, "t", "1"),
	)
}

func TestSlackNotifierEscapesTheName(t *testing.T) {
	var posted []string
	n := adapters.SlackNotifier{
		Send: func(_ context.Context, text string) { posted = append(posted, text) },
	}

	n.Created(context.Background(), "<!channel> bot")
	n.JoinedFamily(context.Background(), "a & b")

	assert.Equal(t, []string{
		":key: An agent account was created: &lt;!channel&gt; bot",
		":family: An agent account joined a family: a &amp; b",
	}, posted)
}

type userReader struct {
	user *models.User
	err  error
}

func (u userReader) GetByID(context.Context, string) (*models.User, error) {
	return u.user, u.err
}

func dbUser(role models.Role) *models.User {
	return &models.User{
		ID: "u1", Email: "db@example.com", DisplayName: "DB Name", Role: role,
		AppAccess: nil, WriteAccess: nil, PrincipalID: "", HasMFA: false,
	}
}

func TestUsersProfileReportsTheRoleAsIsAdmin(t *testing.T) {
	got, err := adapters.Users{Repo: userReader{user: dbUser(models.RoleAdmin)}}.
		Profile(context.Background(), "u1")
	require.NoError(t, err)
	assert.Equal(t, "u1", got.ID)
	assert.Equal(t, "db@example.com", got.Email)
	assert.Equal(t, "DB Name", got.DisplayName)
	assert.True(t, got.IsAdmin)

	got, err = adapters.Users{Repo: userReader{user: dbUser(models.RoleAgent)}}.
		Profile(context.Background(), "u1")
	require.NoError(t, err)
	assert.False(t, got.IsAdmin)

	_, err = adapters.Users{Repo: userReader{err: errors.New("gone")}}.
		Profile(context.Background(), "u1")
	assert.Error(t, err)
}

type sessionReader struct {
	user *models.User
	err  error
}

func (s sessionReader) GetUser(context.Context, string) (*models.User, error) {
	return s.user, s.err
}

func TestSessionsOverlayTheDBRecordOnTheSessionUser(t *testing.T) {
	bare := &models.User{
		ID: "u1", Email: "jwt@example.com", DisplayName: "", Role: models.RoleUser,
		AppAccess: nil, WriteAccess: nil, PrincipalID: "", HasMFA: false,
	}
	sessions := adapters.Sessions{
		Sessions: sessionReader{user: bare},
		Users:    userReader{user: dbUser(models.RoleAdmin)},
	}

	got, err := sessions.SessionUser(context.Background(), "tok")

	require.NoError(t, err)
	assert.Equal(t, "db@example.com", got.Email)
	assert.Equal(t, "DB Name", got.DisplayName)
	assert.True(t, got.IsAdmin)
}

func TestSessionsFallBackToTheSessionUserWithoutADBRecord(t *testing.T) {
	bare := &models.User{
		ID: "u1", Email: "jwt@example.com", DisplayName: "", Role: models.RoleUser,
		AppAccess: nil, WriteAccess: nil, PrincipalID: "", HasMFA: false,
	}
	sessions := adapters.Sessions{
		Sessions: sessionReader{user: bare},
		Users:    userReader{err: errors.New("not found")},
	}

	got, err := sessions.SessionUser(context.Background(), "tok")

	require.NoError(t, err)
	assert.Equal(t, "jwt@example.com", got.Email)
	assert.False(t, got.IsAdmin)
}

func TestSessionsRefuseAnInvalidSession(t *testing.T) {
	sessions := adapters.Sessions{
		Sessions: sessionReader{err: errors.New("expired")},
		Users:    userReader{user: dbUser(models.RoleAdmin)},
	}

	_, err := sessions.SessionUser(context.Background(), "tok")

	assert.Error(t, err)
}

func TestDelegateRefusesAMalformedAccountID(t *testing.T) {
	_, err := adapters.Delegations{Store: nil}.
		Delegate(context.Background(), "not-a-uuid", "c1", "human:a")

	assert.Error(t, err)
}
