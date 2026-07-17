package reading

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/reading/internal/models"
	readingv1 "tools.xdoubleu.com/gen/reading/v1"
	"tools.xdoubleu.com/gen/reading/v1/readingv1connect"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// publicConnectHandler serves the read-only shareable profile RPCs. It is
// registered WITHOUT auth middleware: requests are authorized solely by the
// profile share token, resolved to the owning user here.
type publicConnectHandler struct {
	app *Reading
}

var _ readingv1connect.PublicLibraryServiceHandler = (*publicConnectHandler)(nil)

// resolveToken maps a share token to the owning user ID and display name,
// scoped to the books app; unknown or wrong-app tokens surface as
// CodeNotFound to avoid acting as a token oracle.
func (h *publicConnectHandler) resolveToken(
	ctx context.Context,
	token string,
) (string, string, error) {
	if token == "" {
		return "", "", connect.NewError(
			connect.CodeNotFound,
			errors.New("unknown profile"),
		)
	}
	userID, displayName, err := h.app.profileShares.ResolveToken(
		ctx, token, sharedmodels.ProfileAppReading,
	)
	if errors.Is(err, database.ErrResourceNotFound) {
		return "", "", connect.NewError(
			connect.CodeNotFound,
			errors.New("unknown profile"),
		)
	}
	if err != nil {
		return "", "", connect.NewError(connect.CodeInternal, err)
	}
	return userID, displayName, nil
}

func (h *publicConnectHandler) GetSharedLibrary(
	ctx context.Context,
	req *connect.Request[readingv1.GetSharedLibraryRequest],
) (*connect.Response[readingv1.GetSharedLibraryResponse], error) {
	userID, displayName, err := h.resolveToken(ctx, req.Msg.Token)
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
	return connect.NewResponse(&readingv1.GetSharedLibraryResponse{
		Library: &readingv1.LibraryResponse{
			Reading:  protoUserBooks(data.Reading, base),
			Wishlist: protoUserBooks(data.Wishlist, base),
			Finished: protoUserBooks(data.Finished, base),
			Shelves:  protoBookshelves(data.Shelves, base),
		},
		LastSyncedAt: lastSyncedAt,
		DisplayName:  displayName,
	}), nil
}

func (h *publicConnectHandler) GetSharedBooksProgress(
	ctx context.Context,
	req *connect.Request[readingv1.GetSharedBooksProgressRequest],
) (*connect.Response[readingv1.GetSharedBooksProgressResponse], error) {
	userID, _, err := h.resolveToken(ctx, req.Msg.Token)
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

	return connect.NewResponse(&readingv1.GetSharedBooksProgressResponse{
		Progress: &readingv1.BooksProgressResponse{
			Labels:    labels,
			Values:    values,
			DateStart: dateStart.Format(models.ProgressDateFormat),
			DateEnd:   dateEnd.Format(models.ProgressDateFormat),
		},
	}), nil
}
