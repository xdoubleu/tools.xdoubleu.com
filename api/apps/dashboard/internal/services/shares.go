package services

import (
	"context"
	"errors"

	"tools.xdoubleu.com/internal/kit/database"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// ErrUnknownDashboard is returned for an empty, unknown or wrong-kind share
// token, so callers can't tell those apart (no token oracle).
var ErrUnknownDashboard = errors.New("unknown dashboard")

// ShareService resolves share tokens to the dashboards' owners.
type ShareService struct {
	tokens ShareTokens
}

// NewShareService returns a ShareService over tokens.
func NewShareService(tokens ShareTokens) *ShareService {
	return &ShareService{tokens: tokens}
}

// Resolve maps a share token of kind to its owner's ID and display name.
func (s *ShareService) Resolve(
	ctx context.Context,
	token string,
	kind sharedmodels.DashboardKind,
) (string, string, error) {
	if token == "" {
		return "", "", ErrUnknownDashboard
	}
	userID, displayName, err := s.tokens.ResolveToken(ctx, token, kind)
	if errors.Is(err, database.ErrResourceNotFound) {
		return "", "", ErrUnknownDashboard
	}
	if err != nil {
		return "", "", err
	}
	return userID, displayName, nil
}
