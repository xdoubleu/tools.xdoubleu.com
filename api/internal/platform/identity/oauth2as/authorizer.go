package oauth2as

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"time"
)

// Errors Subject reports; the HTTP adapter turns them into OAuth errors.
var (
	ErrDelegationForbidden = errors.New(
		"only an admin can delegate to an agent account",
	)
	ErrDelegationFailed   = errors.New("the agent account can't be delegated to")
	ErrUndelegationFailed = errors.New("the client's delegation can't be ended")
)

// Clock tells the time.
type Clock interface {
	Now() time.Time
}

// SystemClock is the wall clock.
type SystemClock struct{}

// Now is the current time.
func (SystemClock) Now() time.Time { return time.Now() }

// AgentAccounts is the agent accounts as the consent flow sees them (adr-0048).
type AgentAccounts interface {
	// Delegate binds clientID to the agent account and returns the agent's user ID.
	Delegate(ctx context.Context, accountID, clientID, delegatedBy string) (string, error)
	// Undelegate ends the client's delegation when it is authorized as a person.
	Undelegate(ctx context.Context, clientID string) error
}

// Users reads a user's profile.
type Users interface {
	Profile(ctx context.Context, id string) (ResolvedUser, error)
}

// Authorizer holds the consent rules: who a client is authorized as, which
// approvals are remembered, and the consent token an approval needs.
type Authorizer struct {
	agents AgentAccounts
	users  Users
	grants ConsentGrants
	secret []byte
	clock  Clock
	logger *slog.Logger
}

// NewAuthorizer builds an Authorizer. agents and grants may be nil: then no
// one can delegate and no approval is remembered. secret signs consent tokens.
func NewAuthorizer(
	agents AgentAccounts,
	users Users,
	grants ConsentGrants,
	secret string,
	clock Clock,
	logger *slog.Logger,
) *Authorizer {
	return &Authorizer{
		agents: agents, users: users, grants: grants,
		secret: []byte(secret), clock: clock, logger: logger,
	}
}

// NewConsentToken is what the consent page presents to approve the request q.
func (a *Authorizer) NewConsentToken(userID string, q url.Values) string {
	return newConsentToken(a.secret, userID, q, a.clock.Now())
}

// VerifyConsent reports whether token is a live consent token for this user and
// request.
func (a *Authorizer) VerifyConsent(userID string, q url.Values, token string) bool {
	return verifyConsentToken(a.secret, userID, q, token, a.clock.Now())
}

// Subject is who the approved client acts as: the agent account an admin
// delegated it to, or the approving user.
func (a *Authorizer) Subject(
	ctx context.Context, user ResolvedUser, clientID, accountID string,
) (ResolvedUser, error) {
	if accountID == "" {
		if a.agents == nil {
			return user, nil
		}
		if err := a.agents.Undelegate(ctx, clientID); err != nil {
			return ResolvedUser{}, fmt.Errorf("%w: %w", ErrUndelegationFailed, err)
		}
		return user, nil
	}
	if a.agents == nil || !user.IsAdmin {
		return ResolvedUser{}, ErrDelegationForbidden
	}
	agent, err := a.delegate(ctx, user, clientID, accountID)
	if err != nil {
		return ResolvedUser{}, fmt.Errorf("%w: %w", ErrDelegationFailed, err)
	}
	return agent, nil
}

func (a *Authorizer) delegate(
	ctx context.Context, admin ResolvedUser, clientID, accountID string,
) (ResolvedUser, error) {
	agentID, err := a.agents.Delegate(ctx, accountID, clientID, "human:"+admin.ID)
	if err != nil {
		return ResolvedUser{}, err
	}
	profile, err := a.users.Profile(ctx, agentID)
	if err != nil {
		return ResolvedUser{}, err
	}
	profile.IsAdmin = false
	return profile, nil
}

// Remembered reports whether the user already approved this client for every
// scope, so the consent page can be skipped. A delegation is never remembered,
// and any failure means "no".
func (a *Authorizer) Remembered(
	ctx context.Context, userID, clientID string, scopes []string, accountID string,
) bool {
	if a.grants == nil || accountID != "" {
		return false
	}
	covered, err := a.grants.Covers(ctx, userID, clientID, scopes)
	if err != nil {
		a.logger.WarnContext(ctx, "oauth2 consent grant lookup failed", "error", err)
		return false
	}
	return covered
}

// Remember stores a person's own approval. It is best effort: a failure only
// means consent shows again next time.
func (a *Authorizer) Remember(
	ctx context.Context, userID, clientID string, scopes []string, accountID string,
) {
	if a.grants == nil || accountID != "" {
		return
	}
	if err := a.grants.Remember(ctx, userID, clientID, scopes); err != nil {
		a.logger.WarnContext(ctx, "oauth2 consent grant not stored", "error", err)
	}
}
