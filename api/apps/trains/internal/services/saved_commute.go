package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/apps/trains/internal/repositories"
)

// ErrInvalidCommute is a caller-fixable request: bad label, origin equal to
// destination, or a stop id that isn't a current station.
var ErrInvalidCommute = errors.New("invalid saved commute")

const maxCommuteLabelLen = 100

// SavedCommutesService backs trains.v1's saved-commute RPCs.
type SavedCommutesService struct {
	repos *repositories.Repositories
}

func NewSavedCommutesService(
	repos *repositories.Repositories,
) *SavedCommutesService {
	return &SavedCommutesService{repos: repos}
}

// List returns the user's saved commutes, ordered.
func (s *SavedCommutesService) List(
	ctx context.Context, userID string,
) ([]models.SavedCommute, error) {
	return s.repos.SavedCommutes.ListByUser(ctx, userID)
}

// Create validates, then appends a commute to the user's list.
func (s *SavedCommutesService) Create(
	ctx context.Context, userID, label, originStopID, destStopID string,
) (models.SavedCommute, error) {
	label = strings.TrimSpace(label)
	if err := s.validate(ctx, label, originStopID, destStopID); err != nil {
		return models.SavedCommute{}, err
	}
	//nolint:exhaustruct // remaining fields set by the repository
	return s.repos.SavedCommutes.Create(ctx, models.SavedCommute{
		UserID:            userID,
		Label:             label,
		OriginStopID:      originStopID,
		DestinationStopID: destStopID,
	})
}

// Update changes an existing commute's label and position.
func (s *SavedCommutesService) Update(
	ctx context.Context, userID string, id uuid.UUID, label string, position int,
) (models.SavedCommute, error) {
	label = strings.TrimSpace(label)
	if label == "" || len([]rune(label)) > maxCommuteLabelLen {
		return models.SavedCommute{}, fmt.Errorf(
			"%w: label must be 1..%d characters", ErrInvalidCommute, maxCommuteLabelLen,
		)
	}
	if position < 0 {
		return models.SavedCommute{}, fmt.Errorf(
			"%w: position must not be negative", ErrInvalidCommute,
		)
	}
	//nolint:exhaustruct // only id/user/label/position are updated
	return s.repos.SavedCommutes.Update(ctx, models.SavedCommute{
		ID: id, UserID: userID, Label: label, Position: position,
	})
}

// Delete removes one of the user's saved commutes.
func (s *SavedCommutesService) Delete(
	ctx context.Context, userID string, id uuid.UUID,
) error {
	return s.repos.SavedCommutes.Delete(ctx, id, userID)
}

func (s *SavedCommutesService) validate(
	ctx context.Context, label, originStopID, destStopID string,
) error {
	if label == "" || len([]rune(label)) > maxCommuteLabelLen {
		return fmt.Errorf(
			"%w: label must be 1..%d characters", ErrInvalidCommute, maxCommuteLabelLen,
		)
	}
	if originStopID == destStopID {
		return fmt.Errorf(
			"%w: origin and destination must differ", ErrInvalidCommute,
		)
	}

	stops, err := s.repos.Feed.AllStops(ctx)
	if err != nil {
		return err
	}
	stations := make(map[string]struct{}, len(stops))
	for _, stop := range stops {
		if stop.LocationType == stationLocationType {
			stations[stop.StopID] = struct{}{}
		}
	}
	if _, ok := stations[originStopID]; !ok {
		return fmt.Errorf("%w: unknown origin station", ErrInvalidCommute)
	}
	if _, ok := stations[destStopID]; !ok {
		return fmt.Errorf("%w: unknown destination station", ErrInvalidCommute)
	}
	return nil
}
