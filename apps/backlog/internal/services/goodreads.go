package services

import (
	"context"
	"fmt"
	"log/slog"

	"tools.xdoubleu.com/apps/backlog/internal/repositories"
	"tools.xdoubleu.com/apps/backlog/pkg/goodreads"
)

type GoodreadsService struct {
	logger       *slog.Logger
	goodreads    *repositories.GoodreadsRepository
	client       goodreads.Client
	integrations *IntegrationsService
}

func (service *GoodreadsService) ImportAllBooks(
	ctx context.Context,
	userID string,
) ([]goodreads.Book, error) {
	creds, err := service.integrations.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	if creds.GoodreadsURL == "" {
		return nil, nil
	}

	goodreadsUserID, err := service.client.GetUserID(creds.GoodreadsURL)
	if err != nil {
		return nil, err
	}

	books, err := service.client.GetBooks(ctx, *goodreadsUserID)
	if err != nil {
		return nil, err
	}

	service.logger.DebugContext(ctx, fmt.Sprintf("saving %d books", len(books)))
	err = service.goodreads.UpsertBooks(ctx, books, userID)
	if err != nil {
		return nil, err
	}

	return books, nil
}

func (service *GoodreadsService) GetWantToRead(
	ctx context.Context,
	userID string,
) ([]goodreads.Book, error) {
	return service.goodreads.GetByShelf(ctx, userID, "want-to-read")
}
