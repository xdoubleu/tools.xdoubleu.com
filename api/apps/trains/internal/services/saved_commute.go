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
// destination, or a stop id that isn't a known station.
var ErrInvalidCommute = errors.New("invalid saved commute")

const maxCommuteLabelLen = 100

// SavedCommutesService backs trains.v1's saved-commute RPCs.
type SavedCommutesService struct {
	repos    *repositories.Repositories
	stations *StationsService
}

func NewSavedCommutesService(
	repos *repositories.Repositories, stations *StationsService,
) *SavedCommutesService {
	return &SavedCommutesService{repos: repos, stations: stations}
}

// List returns the user's saved commutes, ordered.
func (s *SavedCommutesService) List(
	ctx context.Context, userID string,
) ([]models.SavedCommute, error) {
	list, err := s.repos.SavedCommutes.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range list {
		s.fillStations(ctx, &list[i])
	}
	return list, nil
}

// fillStations names a commute's stations; one missing from iRail's list
// keeps its id and empty names rather than dropping the row.
func (s *SavedCommutesService) fillStations(
	ctx context.Context,
	sc *models.SavedCommute,
) {
	if st, ok, err := s.stations.ByID(ctx, sc.OriginStopID); err == nil && ok {
		sc.Origin = st
	}
	if st, ok, err := s.stations.ByID(ctx, sc.DestinationStopID); err == nil && ok {
		sc.Destination = st
	}
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
	created, err := s.repos.SavedCommutes.Create(ctx, models.SavedCommute{
		UserID:            userID,
		Label:             label,
		OriginStopID:      originStopID,
		DestinationStopID: destStopID,
	})
	if err != nil {
		return models.SavedCommute{}, err
	}
	s.fillStations(ctx, &created)
	return created, nil
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
	updated, err := s.repos.SavedCommutes.Update(ctx, models.SavedCommute{
		ID: id, UserID: userID, Label: label, Position: position,
	})
	if err != nil {
		return models.SavedCommute{}, err
	}
	s.fillStations(ctx, &updated)
	return updated, nil
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

	for _, check := range []struct{ id, what string }{
		{originStopID, "origin"}, {destStopID, "destination"},
	} {
		_, ok, err := s.stations.ByID(ctx, check.id)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w: unknown %s station", ErrInvalidCommute, check.what)
		}
	}
	return nil
}
