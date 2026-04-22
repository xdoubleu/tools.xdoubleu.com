package services

import (
	"context"
)

type BacklogSummary struct {
	SteamCount     int
	GoodreadsCount int
}

type BacklogService struct {
	steam     *SteamService
	goodreads *GoodreadsService
}

func (s *BacklogService) GetSummary(
	ctx context.Context,
	userID string,
) (BacklogSummary, error) {
	games, err := s.steam.GetBacklog(ctx, userID)
	if err != nil {
		return BacklogSummary{}, err
	}

	books, err := s.goodreads.GetWantToRead(ctx, userID)
	if err != nil {
		return BacklogSummary{}, err
	}

	return BacklogSummary{
		SteamCount:     len(games),
		GoodreadsCount: len(books),
	}, nil
}
