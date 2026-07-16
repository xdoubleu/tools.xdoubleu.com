package books

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/books/internal/models"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/gen/books/v1/booksv1connect"
)

// publicConnectHandler serves the read-only shareable profile RPCs. It is
// registered WITHOUT auth middleware: requests are authorized solely by the
// profile share token, resolved to the owning user here.
type publicConnectHandler struct {
	app *Books
}

var _ booksv1connect.PublicLibraryServiceHandler = (*publicConnectHandler)(nil)

// resolveToken maps a share token to the owning user ID; unknown tokens
// surface as CodeNotFound to avoid acting as a token oracle.
func (h *publicConnectHandler) resolveToken(
	ctx context.Context,
	token string,
) (string, error) {
	if token == "" {
		return "", connect.NewError(
			connect.CodeNotFound,
			errors.New("unknown profile"),
		)
	}
	userID, err := h.app.profileShares.GetUserIDByToken(ctx, token)
	if errors.Is(err, database.ErrResourceNotFound) {
		return "", connect.NewError(
			connect.CodeNotFound,
			errors.New("unknown profile"),
		)
	}
	if err != nil {
		return "", connect.NewError(connect.CodeInternal, err)
	}
	return userID, nil
}

func (h *publicConnectHandler) GetSharedLibrary(
	ctx context.Context,
	req *connect.Request[booksv1.GetSharedLibraryRequest],
) (*connect.Response[booksv1.GetSharedLibraryResponse], error) {
	userID, err := h.resolveToken(ctx, req.Msg.Token)
	if err != nil {
		return nil, err
	}

	data, err := h.app.buildLibraryData(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	lastSeen, err := h.app.Repositories.KoboDevices.GetLastSeenAt(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	lastSyncedAt := ""
	if lastSeen != nil {
		lastSyncedAt = lastSeen.Format(time.RFC3339)
	}

	base := h.app.clients.PublicAPIBaseURL
	return connect.NewResponse(&booksv1.GetSharedLibraryResponse{
		Library: &booksv1.LibraryResponse{
			Reading:  protoUserBooks(data.Reading, base),
			Wishlist: protoUserBooks(data.Wishlist, base),
			Finished: protoUserBooks(data.Finished, base),
			Shelves:  protoBookshelves(data.Shelves, base),
		},
		LastSyncedAt: lastSyncedAt,
	}), nil
}

func (h *publicConnectHandler) GetSharedBooksProgress(
	ctx context.Context,
	req *connect.Request[booksv1.GetSharedBooksProgressRequest],
) (*connect.Response[booksv1.GetSharedBooksProgressResponse], error) {
	userID, err := h.resolveToken(ctx, req.Msg.Token)
	if err != nil {
		return nil, err
	}

	dateStart, dateEnd := parseDateRangeFromStrings(req.Msg.DateStart, req.Msg.DateEnd)
	labels, values, err := h.app.Services.Progress.GetByDates(
		ctx, userID, dateStart, dateEnd,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&booksv1.GetSharedBooksProgressResponse{
		Progress: &booksv1.BooksProgressResponse{
			Labels:    labels,
			Values:    values,
			DateStart: dateStart.Format(models.ProgressDateFormat),
			DateEnd:   dateEnd.Format(models.ProgressDateFormat),
		},
	}), nil
}
