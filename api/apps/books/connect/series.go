package connect

import (
	"context"
	"strings"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/apps/books/internal/protomap"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func (h *Handler) GetSeries(
	ctx context.Context,
	req *booksv1.GetSeriesRequest,
) (*booksv1.GetSeriesResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"series name is required",
		)
	}

	view, err := h.services.Books.GetSeries(ctx, user.ID, name)
	if err != nil {
		return nil, mapError(err)
	}
	return protomap.SeriesResponse(view, h.coverBaseURL), nil
}
