package books

import (
	"context"
	"errors"
	"math"
	"strings"

	"connectrpc.com/connect"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func (h *booksConnectHandler) GetSeries(
	ctx context.Context,
	req *connect.Request[booksv1.GetSeriesRequest],
) (*connect.Response[booksv1.GetSeriesResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	name := strings.TrimSpace(req.Msg.Name)
	if name == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("series name is required"),
		)
	}

	view, err := h.app.Services.Books.GetSeries(ctx, user.ID, name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(
		protoSeries(view, h.app.clients.PublicAPIBaseURL),
	), nil
}

func protoSeries(
	view *services.SeriesView,
	coverBaseURL string,
) *booksv1.GetSeriesResponse {
	entries := make([]*booksv1.SeriesEntry, len(view.Entries))
	for i, e := range view.Entries {
		entry := &booksv1.SeriesEntry{
			Position: e.Position,
		}
		if e.UserBook != nil {
			entry.UserBook = protoUserBook(*e.UserBook, coverBaseURL)
		}
		if e.External != nil {
			entry.External = protoExternalBook(*e.External)
		}
		entries[i] = entry
	}
	return &booksv1.GetSeriesResponse{
		Name:                view.Name,
		Total:               int32FromIntPtr(view.Total),
		Entries:             entries,
		ExternalUnavailable: view.ExternalUnavailable,
	}
}

// seriesFromProto trims the name (empty means no series) and drops a
// position that isn't a finite non-negative number.
func seriesFromProto(name string, position *float64) *models.BookSeries {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if position != nil &&
		(*position < 0 || math.IsNaN(*position) || math.IsInf(*position, 0)) {
		position = nil
	}
	return &models.BookSeries{Name: name, Position: position, Total: nil}
}
