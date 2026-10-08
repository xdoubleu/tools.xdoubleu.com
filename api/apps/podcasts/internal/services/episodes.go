package services

import (
	"context"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/podcasts/internal/models"
	"tools.xdoubleu.com/apps/podcasts/internal/repositories"
	"tools.xdoubleu.com/internal/pagination"
)

type EpisodeService struct {
	repo *repositories.EpisodesRepository
}

// List returns the user's episodes newest first, optionally of one show, and
// whether more follow.
func (s *EpisodeService) List(
	ctx context.Context,
	userID string,
	showID string,
	limit, offset int32,
) ([]models.ListedEpisode, bool, error) {
	var show *uuid.UUID
	if showID != "" {
		id, err := uuid.Parse(showID)
		if err != nil {
			return nil, false, badRequest("invalid show id")
		}
		show = &id
	}

	pageSize, sqlLimit := pagination.Clamp(limit)
	rows, err := s.repo.List(ctx, userID, show, sqlLimit, max(int(offset), 0))
	if err != nil {
		return nil, false, err
	}
	page, more := pagination.Split(rows, pageSize)
	return page, more, nil
}
