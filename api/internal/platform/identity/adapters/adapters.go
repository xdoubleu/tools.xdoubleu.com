// Package adapters implements the identity services' ports over the rest of
// the platform: the auth service, the user directory and Slack.
package adapters

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/account"
	"tools.xdoubleu.com/internal/platform/auth"
	"tools.xdoubleu.com/internal/platform/identity/agentaccounts"
	"tools.xdoubleu.com/internal/platform/identity/oauth2as"
	"tools.xdoubleu.com/internal/platform/slack"
)

// FactorChecker checks a TOTP or recovery code for a session.
type FactorChecker interface {
	VerifyCurrentFactor(ctx context.Context, accessToken, code string) error
}

// StepUp is agentaccounts.StepUp over the auth service, translating its
// lockout and wrong-code errors.
type StepUp struct {
	Auth FactorChecker
}

var _ agentaccounts.StepUp = (*StepUp)(nil)

// VerifyCurrentFactor checks the code for the session behind accessToken.
func (s StepUp) VerifyCurrentFactor(
	ctx context.Context,
	accessToken, code string,
) error {
	err := s.Auth.VerifyCurrentFactor(ctx, accessToken, code)
	switch {
	case errors.Is(err, auth.ErrTooManyAttempts):
		return translated{error: err, kind: agentaccounts.ErrTooManyAttempts}
	case errors.Is(err, auth.ErrWrongCredential):
		return translated{error: err, kind: agentaccounts.ErrWrongCredential}
	}
	return err
}

// translated keeps the auth error's message and chain and also matches kind.
type translated struct {
	error
	kind error
}

func (t translated) Is(target error) bool { return target == t.kind }

func (t translated) Unwrap() error { return t.error }

// SlackNotifier is agentaccounts.Notifier posting to Slack through send.
type SlackNotifier struct {
	Send func(ctx context.Context, text string)
}

var _ agentaccounts.Notifier = (*SlackNotifier)(nil)

// Created posts that an account was created.
func (n SlackNotifier) Created(ctx context.Context, name string) {
	n.Send(ctx, ":key: An agent account was created: "+slack.Escape(name))
}

// JoinedFamily posts that an account joined a family.
func (n SlackNotifier) JoinedFamily(ctx context.Context, name string) {
	n.Send(ctx, ":family: An agent account joined a family: "+slack.Escape(name))
}

// Delegations is oauth2as.AgentAccounts over the agent account store.
type Delegations struct {
	Store *agentaccounts.Store
}

var _ oauth2as.AgentAccounts = (*Delegations)(nil)

// Delegate binds the client to the agent account and returns the agent's user ID.
func (d Delegations) Delegate(
	ctx context.Context, accountID, clientID, delegatedBy string,
) (string, error) {
	id, err := uuid.Parse(accountID)
	if err != nil {
		return "", err
	}
	acct, err := d.Store.Delegate(ctx, id, clientID, delegatedBy)
	if err != nil {
		return "", err
	}
	return acct.UserID.String(), nil
}

// Undelegate ends the client's delegation.
func (d Delegations) Undelegate(ctx context.Context, clientID string) error {
	return d.Store.Undelegate(ctx, clientID)
}

// UserReader reads a user's DB record.
type UserReader interface {
	GetByID(ctx context.Context, id string) (*models.User, error)
}

// Users is oauth2as.Users over the app users repository.
type Users struct {
	Repo UserReader
}

var _ oauth2as.Users = (*Users)(nil)

// Profile is the user's identity for ID-token claims.
func (u Users) Profile(ctx context.Context, id string) (oauth2as.ResolvedUser, error) {
	user, err := u.Repo.GetByID(ctx, id)
	if err != nil {
		return oauth2as.ResolvedUser{}, err
	}
	return resolved(user), nil
}

func resolved(user *models.User) oauth2as.ResolvedUser {
	return oauth2as.ResolvedUser{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		IsAdmin:     user.Role == models.RoleAdmin,
	}
}

// SessionReader resolves an access token to its user.
type SessionReader interface {
	GetUser(ctx context.Context, accessToken string) (*models.User, error)
}

// Sessions resolves a browser session with the DB role and display name overlaid.
type Sessions struct {
	Sessions SessionReader
	Users    UserReader
}

// SessionUser resolves accessToken to a user.
func (s Sessions) SessionUser(
	ctx context.Context, accessToken string,
) (oauth2as.ResolvedUser, error) {
	user, err := s.Sessions.GetUser(ctx, accessToken)
	if err != nil {
		return oauth2as.ResolvedUser{}, err
	}
	// The bare auth-schema user always has RoleUser and no display name.
	if dbUser, dbErr := s.Users.GetByID(ctx, user.ID); dbErr == nil {
		return resolved(dbUser), nil
	}
	return resolved(user), nil
}

var _ UserReader = (*account.AppUsersRepository)(nil)
