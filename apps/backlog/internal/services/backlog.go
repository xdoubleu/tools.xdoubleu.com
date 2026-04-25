package services

import (
	"context"

	"tools.xdoubleu.com/apps/backlog/internal/models"
)

type BacklogSummary struct {
	SteamCount int
	BooksCount int
}

type BacklogService struct {
	steam *SteamService
	books *BookService
}

func (s *BacklogService) GetSummary(
	ctx context.Context,
	userID string,
) (BacklogSummary, error) {
	games, err := s.steam.GetBacklog(ctx, userID)
	if err != nil {
		return BacklogSummary{}, err
	}

	wishlist, err := s.books.GetByStatus(ctx, userID, models.StatusToRead)
	if err != nil {
		return BacklogSummary{}, err
	}

	return BacklogSummary{
		SteamCount: len(games),
		BooksCount: len(wishlist),
	}, nil
}
