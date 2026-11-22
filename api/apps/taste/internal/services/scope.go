package services

import (
	"context"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/internal/consumption"
)

// SharedApp is the only app the family-wide taste reads.
const SharedApp = "movies"

// AppAccess reports whether the caller behind ctx may read an app, so taste
// never surfaces an app's data to someone without access to it.
type AppAccess interface {
	CanAccess(ctx context.Context, appName string) bool
}

// familyStore resolves a user's family, creating a family-of-one on demand.
type familyStore interface {
	EnsureFamily(ctx context.Context, userID string) (uuid.UUID, error)
}

// Scope is what a request covers: the subject whose log and picks it reads,
// which is the caller or, when shared, their family; and the apps it may read.
type Scope struct {
	// User is always the caller.
	User    string
	Subject string
	Access  func(appName string) bool
}

// ScopeService resolves the scope of a request.
type ScopeService struct {
	family familyStore
	access AppAccess
}

func NewScopeService(family familyStore, access AppAccess) *ScopeService {
	return &ScopeService{family: family, access: access}
}

// Resolve returns the scope of userID's request. The shared one covers only
// the movies app, whose shared list is the one family-wide log.
func (s *ScopeService) Resolve(
	ctx context.Context,
	userID string,
	shared bool,
) (Scope, error) {
	access := func(appName string) bool {
		return s.access.CanAccess(ctx, appName)
	}
	if !shared {
		return Scope{User: userID, Subject: userID, Access: access}, nil
	}
	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return Scope{}, err
	}
	return Scope{
		User:    userID,
		Subject: consumption.SharedSubject(familyID),
		Access: func(appName string) bool {
			return appName == SharedApp && access(appName)
		},
	}, nil
}
