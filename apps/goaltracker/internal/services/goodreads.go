package services

import (
	"context"
	"fmt"
	"log/slog"

	"tools.xdoubleu.com/apps/goaltracker/internal/repositories"
	"tools.xdoubleu.com/apps/goaltracker/pkg/goodreads"
)

type GoodreadsService struct {
	logger     *slog.Logger
	goodreads  *repositories.GoodreadsRepository
	client     goodreads.Client
	profileURL string
}

func (service *GoodreadsService) ImportAllBooks(
	ctx context.Context,
	userID string,
) ([]goodreads.Book, error) {
	goodreadsUserID, err := service.client.GetUserID(service.profileURL)
	if err != nil {
		return nil, err
	}

	books, err := service.client.GetBooks(*goodreadsUserID)
	if err != nil {
		return nil, err
	}

	service.logger.Debug(fmt.Sprintf("saving %d books", len(books)))
	err = service.goodreads.UpsertBooks(ctx, books, userID)
	if err != nil {
		return nil, err
	}

	return books, nil
}

func (service *GoodreadsService) GetAllBooks(
	ctx context.Context,
	userID string,
) ([]goodreads.Book, error) {
	return service.goodreads.GetAllBooks(ctx, userID)
}

func (service *GoodreadsService) GetAllTags(
	ctx context.Context,
	userID string,
) ([]string, error) {
	return service.goodreads.GetAllTags(ctx, userID)
}

func (service *GoodreadsService) GetBooksByTag(
	ctx context.Context,
	tag string,
	userID string,
) ([]goodreads.Book, error) {
	return service.goodreads.GetBooksByTag(ctx, tag, userID)
}

func (service *GoodreadsService) GetBooksByIDs(
	ctx context.Context,
	ids []int64,
	userID string,
) ([]goodreads.Book, error) {
	return service.goodreads.GetBooksByIDs(ctx, ids, userID)
}
