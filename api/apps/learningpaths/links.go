package learningpaths

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books"
	"tools.xdoubleu.com/apps/feeds"
	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	"tools.xdoubleu.com/apps/learningpaths/internal/services"
)

// BookSource is the exported books API learningpaths links resources to.
type BookSource interface {
	GetLibraryBookByID(
		ctx context.Context, userID string, bookID uuid.UUID,
	) (*books.LibraryBook, error)
	EnsureLibraryBook(
		ctx context.Context, userID, provider, providerID, shelf string,
	) (uuid.UUID, error)
}

// FeedItemSource is the exported feeds API learningpaths links resources to.
type FeedItemSource interface {
	GetItemByID(
		ctx context.Context, userID string, itemID uuid.UUID,
	) (*feeds.SharedItem, error)
}

// bookPort adapts BookSource to the services.BookLookup port.
type bookPort struct{ src BookSource }

func (p bookPort) GetLibraryBook(
	ctx context.Context, userID string, bookID uuid.UUID,
) (*models.LinkedBook, error) {
	b, err := p.src.GetLibraryBookByID(ctx, userID, bookID)
	if err != nil {
		return nil, err
	}
	return &models.LinkedBook{
		Title:           b.Title,
		Status:          b.Status,
		ProgressPercent: b.ProgressPercent,
		CoverURL:        b.CoverURL,
	}, nil
}

func (p bookPort) EnsureLibraryBook(
	ctx context.Context, userID, provider, providerID, shelf string,
) (uuid.UUID, error) {
	id, err := p.src.EnsureLibraryBook(ctx, userID, provider, providerID, shelf)
	if errors.Is(err, books.ErrExternalBookNotFound) {
		return uuid.Nil, services.ErrExternalBookNotFound
	}
	return id, err
}

// feedPort adapts FeedItemSource to the services.FeedItemLookup port.
type feedPort struct{ src FeedItemSource }

func (p feedPort) GetFeedItem(
	ctx context.Context, userID string, itemID uuid.UUID,
) (*models.LinkedFeedItem, error) {
	it, err := p.src.GetItemByID(ctx, userID, itemID)
	if err != nil {
		return nil, err
	}
	return &models.LinkedFeedItem{
		Title:      it.Title,
		SourceURL:  it.SourceURL,
		Read:       it.Read,
		Bookmarked: it.Bookmarked,
	}, nil
}
